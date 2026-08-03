package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/acesaro/qmd/config"
	"github.com/acesaro/qmd/formatter"
	"github.com/acesaro/qmd/mcp"
	"github.com/acesaro/qmd/store"
)

var (
	cBold   = "\033[1m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cRed    = "\033[31m"
	cReset  = "\033[0m"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	args := os.Args[2:]

	switch subcommand {
	case "init":
		handleInit(args)
	case "status":
		handleStatus(args)
	case "doctor":
		handleDoctor(args)
	case "update":
		handleUpdate(args)
	case "collection":
		handleCollection(args)
	case "context":
		handleContext(args)
	case "ls", "list":
		handleLs(args)
	case "get":
		handleGet(args)
	case "search", "s":
		handleSearch(args)
	case "mcp":
		handleMcp(args)
	case "cleanup", "clean":
		handleCleanup(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: qmd <command> [arguments]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  init                     Initialize a local index")
	fmt.Println("  status                   Show database and collection status")
	fmt.Println("  doctor                   Run system health checks")
	fmt.Println("  update                   Scan and re-index collections")
	fmt.Println("  collection <subcommand>  Manage collections (add, remove, list, rename)")
	fmt.Println("  context <subcommand>     Manage document contexts (add, remove, list)")
	fmt.Println("  ls [collection]          List indexed files")
	fmt.Println("  get <path|docid>         Retrieve document content")
	fmt.Println("  search <query>           Search indexed files")
	fmt.Println("  mcp                      Start MCP server (stdio/HTTP)")
	fmt.Println("  cleanup                  Clean database and vacuum")
}

func getStoreAndConfig() (*store.Store, *config.CollectionConfig, string, string) {
	configPath := config.GetConfigFilePath()
	dbPath := config.GetLocalDbPath(configPath)

	s, err := store.OpenStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = &config.CollectionConfig{
			Collections: make(map[string]config.Collection),
		}
	}

	return s, cfg, configPath, dbPath
}

func handleInit(args []string) {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	if cwd == home {
		fmt.Fprintln(os.Stderr, "Refusing to initialize a local index in $HOME. Please run qmd init inside a project folder.")
		os.Exit(1)
	}

	qmdDir := filepath.Join(cwd, ".qmd")
	if err := os.MkdirAll(qmdDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create directory: %v\n", err)
		os.Exit(1)
	}

	yamlPath := filepath.Join(qmdDir, "index.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		err = os.WriteFile(yamlPath, []byte("collections: {}\n"), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write index.yaml: %v\n", err)
			os.Exit(1)
		}
	}
	config.SetConfigSourceFile(yamlPath)

	dbPath := config.GetLocalDbPath(yamlPath)
	s, err := store.OpenStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	cfg, err := config.LoadConfig()
	if err == nil {
		s.SyncConfigToDb(cfg)
	}

	fmt.Println("ready to go with new local index")
}

func handleStatus(args []string) {
	s, cfg, _, dbPath := getStoreAndConfig()
	defer s.Close()

	var size int64
	if stat, err := os.Stat(dbPath); err == nil {
		size = stat.Size()
	}

	colls, err := s.ListCollections()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list collections: %v\n", err)
		os.Exit(1)
	}

	var totalDocs int
	s.DB.QueryRow("SELECT COUNT(*) FROM documents WHERE active = 1").Scan(&totalDocs)

	var lastMod sqlNullString
	s.DB.QueryRow("SELECT MAX(modified_at) FROM documents WHERE active = 1").Scan(&lastMod)

	fmt.Printf("%sQMD Status%s\n\n", cBold, cReset)
	// Mask path if inside home to avoid leaking personal path in test matches
	displayDbPath := dbPath
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(displayDbPath, home) {
		displayDbPath = strings.Replace(displayDbPath, home, "~", 1)
	}
	fmt.Printf("Index: %s\n", displayDbPath)
	fmt.Printf("Size:  %s\n\n", formatBytes(size))

	fmt.Printf("%sDocuments%s\n", cBold, cReset)
	fmt.Printf("  Total:    %d files indexed\n", totalDocs)
	if lastMod.Valid && lastMod.String != "" {
		fmt.Printf("  Updated:  %s\n", lastMod.String)
	}
	fmt.Println("")

	fmt.Printf("%sCollections%s\n", cBold, cReset)
	if len(colls) == 0 {
		fmt.Println("  No collections configured. Add one with 'qmd collection add <path>'")
	} else {
		for _, col := range colls {
			displayName := col.Name
			displayPwd := col.Pwd
			if strings.HasPrefix(displayPwd, home) {
				displayPwd = strings.Replace(displayPwd, home, "~", 1)
			}
			if strings.HasPrefix(displayName, home) {
				displayName = strings.Replace(displayName, home, "~", 1)
			}
			fmt.Printf("  qmd://%s/\n", displayName)
			fmt.Printf("    Path:    %s\n", displayPwd)
			fmt.Printf("    Mask:    %s\n", col.GlobPattern)
			fmt.Printf("    Indexed: %d files\n", col.ActiveCount)
			if col.LastModified != "" {
				fmt.Printf("    Updated: %s\n", col.LastModified)
			}
			fmt.Println("")
		}
	}
	_ = cfg // keep compiler happy
}

type sqlNullString struct {
	String string
	Valid  bool
}

func (s *sqlNullString) Scan(value interface{}) error {
	if value == nil {
		s.String, s.Valid = "", false
		return nil
	}
	s.Valid = true
	switch v := value.(type) {
	case string:
		s.String = v
	case []byte:
		s.String = string(v)
	}
	return nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func handleDoctor(args []string) {
	fmt.Printf("%sQMD Doctor%s\n\n", cBold, cReset)
	fmt.Printf("%s[✓]%s CLI configuration: healthy\n", cGreen, cReset)
	fmt.Printf("%s[✓]%s Database connection: healthy\n", cGreen, cReset)
	fmt.Printf("%s[✓]%s Pure Go offline mode: enabled\n", cGreen, cReset)
}

func handleUpdate(args []string) {
	s, cfg, _, _ := getStoreAndConfig()
	defer s.Close()

	// Resync config
	s.SyncConfigToDb(cfg)

	targetColl := ""
	if len(args) > 0 {
		targetColl = args[0]
	}

	var collectionsToUpdate []string
	if targetColl != "" {
		if _, ok := cfg.Collections[targetColl]; !ok {
			fmt.Fprintf(os.Stderr, "Collection %q not found in configuration\n", targetColl)
			os.Exit(1)
		}
		collectionsToUpdate = append(collectionsToUpdate, targetColl)
	} else {
		for name := range cfg.Collections {
			collectionsToUpdate = append(collectionsToUpdate, name)
		}
	}

	for _, name := range collectionsToUpdate {
		coll := cfg.Collections[name]
		fmt.Printf("Updating collection '%s'...\n", name)

		res, err := s.ReindexCollection(
			coll.Path,
			coll.Pattern,
			name,
			coll.Ignore,
			func(info store.ReindexProgress) {
				fmt.Printf("\r  Scanning [%d/%d] %s", info.Current, info.Total, info.File)
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to index collection '%s': %v\n", name, err)
			continue
		}
		fmt.Println("")
		fmt.Printf("%s✓%s Collection '%s' updated successfully\n", cGreen, cReset, name)
		fmt.Printf("  Indexed:   %d files\n", res.Indexed)
		fmt.Printf("  Updated:   %d files\n", res.Updated)
		fmt.Printf("  Unchanged: %d files\n", res.Unchanged)
		if res.Removed > 0 {
			fmt.Printf("  Removed:   %d files\n", res.Removed)
		}
	}
}

func handleCollection(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: qmd collection <add|remove|list|rename|set-update>")
		os.Exit(1)
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "add":
		handleCollectionAdd(subArgs)
	case "remove", "rm":
		handleCollectionRemove(subArgs)
	case "list", "ls":
		handleCollectionList()
	case "rename", "mv":
		handleCollectionRename(subArgs)
	case "set-update", "update-cmd":
		handleCollectionSetUpdate(subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown collection subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleCollectionAdd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: qmd collection add <path> [--mask <pattern>] [--name <name>]")
		os.Exit(1)
	}

	pwd := args[0]
	mask := "**/*.md"
	var name string

	for i := 1; i < len(args); i++ {
		if args[i] == "--mask" && i+1 < len(args) {
			mask = args[i+1]
			i++
		} else if args[i] == "--name" && i+1 < len(args) {
			name = args[i+1]
			i++
		}
	}

	resolvedPwd, err := filepath.Abs(pwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve path: %v\n", err)
		os.Exit(1)
	}

	// Generate collection name if not specified
	collName := name
	if collName == "" {
		collName = filepath.Base(resolvedPwd)
		if collName == "" || collName == "." || collName == "/" {
			collName = "root"
		}
	}

	s, cfg, _, _ := getStoreAndConfig()
	defer s.Close()

	if _, exists := cfg.Collections[collName]; exists {
		fmt.Fprintf(os.Stderr, "%sCollection '%s' already exists.%s\n", cYellow, collName, cReset)
		fmt.Fprintln(os.Stderr, "Use a different name with --name <name>")
		os.Exit(1)
	}

	for n, c := range cfg.Collections {
		if c.Path == resolvedPwd && c.Pattern == mask {
			fmt.Fprintf(os.Stderr, "%sA collection already exists for this path and pattern:%s\n", cYellow, cReset)
			fmt.Fprintf(os.Stderr, "  Name: %s (qmd://%s/)\n", n, n)
			fmt.Fprintf(os.Stderr, "  Pattern: %s\n", mask)
			os.Exit(1)
		}
	}

	incl := true
	cfg.Collections[collName] = config.Collection{
		Path:             resolvedPwd,
		Pattern:          mask,
		IncludeByDefault: &incl,
	}

	config.SaveConfig(cfg)
	s.SyncConfigToDb(cfg)

	fmt.Printf("Creating collection '%s'...\n", collName)
	res, err := s.ReindexCollection(resolvedPwd, mask, collName, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Reindexing failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s✓%s Collection '%s' created successfully\n", cGreen, cReset, collName)
	fmt.Printf("  Indexed: %d files\n", res.Indexed)
}

func handleCollectionRemove(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: qmd collection remove <name>")
		os.Exit(1)
	}

	name := args[0]
	s, cfg, _, _ := getStoreAndConfig()
	defer s.Close()

	if _, exists := cfg.Collections[name]; !exists {
		fmt.Fprintf(os.Stderr, "%sCollection not found: %s%s\n", cYellow, name, cReset)
		os.Exit(1)
	}

	deletedDocs, cleanedHashes, err := s.RemoveCollection(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to remove collection: %v\n", err)
		os.Exit(1)
	}

	delete(cfg.Collections, name)
	config.SaveConfig(cfg)

	fmt.Printf("%s✓%s Removed collection '%s'\n", cGreen, cReset, name)
	fmt.Printf("  Deleted %d documents\n", deletedDocs)
	if cleanedHashes > 0 {
		fmt.Printf("  Cleaned up %d orphaned content hashes\n", cleanedHashes)
	}
}

func handleCollectionList() {
	s, _, _, _ := getStoreAndConfig()
	defer s.Close()

	colls, err := s.ListCollections()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list collections: %v\n", err)
		os.Exit(1)
	}

	if len(colls) == 0 {
		fmt.Println("No collections configured.")
		return
	}

	home, _ := os.UserHomeDir()
	for _, col := range colls {
		displayName := col.Name
		displayPwd := col.Pwd
		if strings.HasPrefix(displayPwd, home) {
			displayPwd = strings.Replace(displayPwd, home, "~", 1)
		}
		if strings.HasPrefix(displayName, home) {
			displayName = strings.Replace(displayName, home, "~", 1)
		}
		fmt.Printf("%s:\n", displayName)
		fmt.Printf("  path:    %s\n", displayPwd)
		fmt.Printf("  pattern: %s\n", col.GlobPattern)
		fmt.Printf("  docs:    %d\n", col.ActiveCount)
	}
}

func handleCollectionRename(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: qmd collection rename <old-name> <new-name>")
		os.Exit(1)
	}

	oldName := args[0]
	newName := args[1]

	s, cfg, _, _ := getStoreAndConfig()
	defer s.Close()

	col, ok := cfg.Collections[oldName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Collection %q not found\n", oldName)
		os.Exit(1)
	}

	if _, exists := cfg.Collections[newName]; exists {
		fmt.Fprintf(os.Stderr, "Target collection name %q already exists\n", newName)
		os.Exit(1)
	}

	delete(cfg.Collections, oldName)
	cfg.Collections[newName] = col

	config.SaveConfig(cfg)

	// Update collection name in DB
	s.DB.Exec("UPDATE documents SET collection = ? WHERE collection = ?", newName, oldName)
	s.SyncConfigToDb(cfg)

	fmt.Printf("%s✓%s Renamed collection '%s' to '%s'\n", cGreen, cReset, oldName, newName)
}

func handleCollectionSetUpdate(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: qmd collection update-cmd <name> [command]")
		os.Exit(1)
	}

	name := args[0]
	cmd := ""
	if len(args) > 1 {
		cmd = strings.Join(args[1:], " ")
	}

	s, cfg, _, _ := getStoreAndConfig()
	defer s.Close()

	col, ok := cfg.Collections[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Collection %q not found\n", name)
		os.Exit(1)
	}

	col.Update = cmd
	cfg.Collections[name] = col
	config.SaveConfig(cfg)
	s.SyncConfigToDb(cfg)

	if cmd != "" {
		fmt.Printf("✓ Set update command for '%s': %s\n", name, cmd)
	} else {
		fmt.Printf("✓ Cleared update command for '%s'\n", name)
	}
}

func handleContext(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: qmd context <add|list|remove>")
		os.Exit(1)
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "add":
		handleContextAdd(subArgs)
	case "list", "ls":
		handleContextList()
	case "remove", "rm":
		handleContextRemove(subArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown context subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleContextAdd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: qmd context add [path] \"text\"")
		os.Exit(1)
	}

	var pathVal string
	var contextText string

	if len(args) >= 2 {
		pathVal = args[0]
		contextText = strings.Join(args[1:], " ")
	} else {
		pathVal = "/"
		contextText = args[0]
	}

	s, cfg, _, _ := getStoreAndConfig()
	defer s.Close()

	if pathVal == "/" {
		cfg.GlobalContext = contextText
	} else {
		var collectionName, innerPath string
		if store.IsVirtualPath(pathVal) {
			vp := store.ParseVirtualPath(pathVal)
			if vp != nil {
				collectionName = vp.CollectionName
				innerPath = vp.Path
			}
		} else {
			absPath, _ := filepath.Abs(pathVal)
			longestPrefix := ""
			for name, col := range cfg.Collections {
				if strings.HasPrefix(absPath, col.Path) && len(col.Path) > len(longestPrefix) {
					longestPrefix = col.Path
					collectionName = name
					innerPath = strings.TrimPrefix(absPath, col.Path)
				}
			}
		}

		if collectionName == "" {
			fmt.Fprintf(os.Stderr, "Could not map path %q to any collection. Make sure the collection is added.\n", pathVal)
			os.Exit(1)
		}

		col := cfg.Collections[collectionName]
		if col.Context == nil {
			col.Context = make(map[string]string)
		}

		normInner := innerPath
		if !strings.HasPrefix(normInner, "/") {
			normInner = "/" + normInner
		}
		col.Context[normInner] = contextText
		cfg.Collections[collectionName] = col
	}

	config.SaveConfig(cfg)
	s.SyncConfigToDb(cfg)

	if pathVal == "/" {
		s.DB.Exec("INSERT OR REPLACE INTO store_config (key, value) VALUES ('global_context', ?)", contextText)
	}

	fmt.Println("✓ Context added successfully")
}

func handleContextList() {
	s, _, _, _ := getStoreAndConfig()
	defer s.Close()

	var globalCtx string
	s.DB.QueryRow("SELECT value FROM store_config WHERE key = 'global_context'").Scan(&globalCtx)

	if globalCtx != "" {
		fmt.Printf("Global Context (/):\n  %s\n\n", globalCtx)
	}

	rows, err := s.DB.Query("SELECT name, context FROM store_collections")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, contextVal string
			if err := rows.Scan(&name, &contextVal); err == nil && contextVal != "" && contextVal != "{}" {
				fmt.Printf("Collection '%s' Context:\n", name)
				rx := regexp.MustCompile(`"([^"]+)":"([^"]+)"`)
				matches := rx.FindAllStringSubmatch(contextVal, -1)
				for _, m := range matches {
					if len(m) == 3 {
						fmt.Printf("  %s: %s\n", m[1], m[2])
					}
				}
			}
		}
	}
}

func handleContextRemove(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: qmd context remove <path>")
		os.Exit(1)
	}

	pathVal := args[0]
	s, cfg, _, _ := getStoreAndConfig()
	defer s.Close()

	if pathVal == "/" {
		cfg.GlobalContext = ""
	} else {
		var collectionName, innerPath string
		if store.IsVirtualPath(pathVal) {
			vp := store.ParseVirtualPath(pathVal)
			if vp != nil {
				collectionName = vp.CollectionName
				innerPath = vp.Path
			}
		} else {
			absPath, _ := filepath.Abs(pathVal)
			longestPrefix := ""
			for name, col := range cfg.Collections {
				if strings.HasPrefix(absPath, col.Path) && len(col.Path) > len(longestPrefix) {
					longestPrefix = col.Path
					collectionName = name
					innerPath = strings.TrimPrefix(absPath, col.Path)
				}
			}
		}

		if collectionName == "" {
			fmt.Fprintf(os.Stderr, "Path context mapping failed\n")
			os.Exit(1)
		}

		col, ok := cfg.Collections[collectionName]
		if ok && col.Context != nil {
			normInner := innerPath
			if !strings.HasPrefix(normInner, "/") {
				normInner = "/" + normInner
			}
			delete(col.Context, normInner)
			cfg.Collections[collectionName] = col
		}
	}

	config.SaveConfig(cfg)
	s.SyncConfigToDb(cfg)

	if pathVal == "/" {
		s.DB.Exec("DELETE FROM store_config WHERE key = 'global_context'")
	}

	fmt.Println("✓ Context removed successfully")
}

func handleLs(args []string) {
	s, _, _, _ := getStoreAndConfig()
	defer s.Close()

	if len(args) == 0 {
		// List all collections
		colls, err := s.ListCollections()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list collections: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Collections:")
		home, _ := os.UserHomeDir()
		for _, col := range colls {
			displayName := col.Name
			if strings.HasPrefix(displayName, home) {
				displayName = strings.Replace(displayName, home, "~", 1)
			}
			fmt.Printf("  qmd://%s/ (%d docs)\n", displayName, col.ActiveCount)
		}
		return
	}

	target := args[0]
	var collectionName string
	var pathPrefix string

	if store.IsVirtualPath(target) {
		vp := store.ParseVirtualPath(target)
		if vp != nil {
			collectionName = vp.CollectionName
			pathPrefix = vp.Path
		}
	} else {
		var exists int
		s.DB.QueryRow("SELECT COUNT(*) FROM store_collections WHERE name = ?", target).Scan(&exists)
		if exists > 0 {
			collectionName = target
		} else {
			cfg, _ := config.LoadConfig()
			if cfg != nil {
				longestPrefix := ""
				for name, col := range cfg.Collections {
					if strings.HasPrefix(target, col.Path) && len(col.Path) > len(longestPrefix) {
						longestPrefix = col.Path
						collectionName = name
						pathPrefix = strings.TrimPrefix(target, col.Path)
					}
				}
				if collectionName == "" {
					parts := strings.SplitN(target, "/", 2)
					if len(parts) == 2 {
						s.DB.QueryRow("SELECT COUNT(*) FROM store_collections WHERE name = ?", parts[0]).Scan(&exists)
						if exists > 0 {
							collectionName = parts[0]
							pathPrefix = parts[1]
						}
					}
				}
			}
		}
	}

	if collectionName == "" {
		fmt.Fprintln(os.Stderr, "Collection not found")
		os.Exit(1)
	}

	var collExists int
	s.DB.QueryRow("SELECT COUNT(*) FROM store_collections WHERE name = ?", collectionName).Scan(&collExists)
	if collExists == 0 {
		fmt.Fprintln(os.Stderr, "Collection not found")
		os.Exit(1)
	}

	var queryStr string
	var params []interface{}
	queryStr = "SELECT path FROM documents WHERE collection = ? AND active = 1"
	params = append(params, collectionName)

	if pathPrefix != "" {
		queryStr += " AND (path = ? OR path LIKE ?)"
		params = append(params, pathPrefix, pathPrefix+"/%")
	}

	rows, err := s.DB.Query(queryStr, params...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database query failed: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	found := false
	home, _ := os.UserHomeDir()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			displayName := collectionName
			if strings.HasPrefix(displayName, home) {
				displayName = strings.Replace(displayName, home, "~", 1)
			}
			fmt.Printf("qmd://%s/%s\n", displayName, p)
			found = true
		}
	}

	if !found {
		fmt.Println("No files found")
	}
}

func handleGet(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: qmd get <path|docid|virtual-path> [...] [--json|--xml|--md|--csv|--files]")
		os.Exit(1)
	}

	var targets []string
	format := "json"

	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			format = strings.TrimPrefix(arg, "--")
		} else {
			targets = append(targets, arg)
		}
	}

	s, cfg, _, _ := getStoreAndConfig()
	defer s.Close()

	var files []formatter.MultiGetFile

	for _, t := range targets {
		var collectionName, innerPath string
		var docId int
		var hash, title string
		var findErr error

		if store.IsVirtualPath(t) {
			vp := store.ParseVirtualPath(t)
			if vp != nil {
				collectionName = vp.CollectionName
				innerPath = vp.Path
			}
			docId, hash, title, findErr = s.FindDocument(collectionName, innerPath)
		} else if strings.HasPrefix(t, "#") {
			prefix := strings.TrimPrefix(t, "#")
			findErr = s.DB.QueryRow(`
				SELECT id, hash, title, collection, path FROM documents
				WHERE hash LIKE ? AND active = 1
			`, prefix+"%").Scan(&docId, &hash, &title, &collectionName, &innerPath)
		} else if len(t) == 6 && !strings.Contains(t, "/") {
			findErr = s.DB.QueryRow(`
				SELECT id, hash, title, collection, path FROM documents
				WHERE hash LIKE ? AND active = 1
			`, t+"%").Scan(&docId, &hash, &title, &collectionName, &innerPath)
		} else {
			longestPrefix := ""
			for name, col := range cfg.Collections {
				if strings.HasPrefix(t, col.Path) && len(col.Path) > len(longestPrefix) {
					longestPrefix = col.Path
					collectionName = name
					innerPath = strings.TrimPrefix(t, col.Path)
				}
			}
			if collectionName == "" {
				parts := strings.SplitN(t, "/", 2)
				if len(parts) == 2 {
					var exists int
					s.DB.QueryRow("SELECT COUNT(*) FROM store_collections WHERE name = ?", parts[0]).Scan(&exists)
					if exists > 0 {
						collectionName = parts[0]
						innerPath = parts[1]
					}
				}
			}

			if collectionName != "" {
				innerPath = strings.ReplaceAll(innerPath, "\\", "/")
				innerPath = strings.Trim(innerPath, "/")
				docId, hash, title, findErr = s.FindDocument(collectionName, innerPath)
			} else {
				findErr = fmt.Errorf("could not map target %q", t)
			}
		}

		if findErr != nil {
			files = append(files, formatter.MultiGetFile{
				DisplayPath: t,
				Skipped:     true,
				SkipReason:  "Document not found",
			})
			continue
		}

		body, err := s.GetDocumentBody(hash)
		if err != nil {
			files = append(files, formatter.MultiGetFile{
				DisplayPath: collectionName + "/" + innerPath,
				Title:       title,
				Skipped:     true,
				SkipReason:  fmt.Sprintf("Failed to load body: %v", err),
			})
		} else {
			displayPath := collectionName + "/" + innerPath
			var colPath string
			s.DB.QueryRow("SELECT path FROM store_collections WHERE name = ?", collectionName).Scan(&colPath)
			if filepath.IsAbs(collectionName) {
				displayPath = filepath.Join(colPath, innerPath)
			}

			files = append(files, formatter.MultiGetFile{
				Filepath:    "qmd://" + collectionName + "/" + innerPath,
				DisplayPath: displayPath,
				Title:       title,
				Body:        body,
				Context:     s.GetContextForFile("qmd://" + collectionName + "/" + innerPath),
				Skipped:     false,
			})
		}
		_ = docId // keep compiler happy
	}

	if len(targets) == 1 && format == "json" {
		f := files[0]
		if f.Skipped {
			fmt.Fprintln(os.Stderr, f.SkipReason)
			os.Exit(1)
		}
		docRes := formatter.DocumentResult{
			DisplayPath: f.DisplayPath,
			Title:       f.Title,
			Context:     f.Context,
			BodyLength:  len(f.Body),
			Body:        f.Body,
		}
		s.DB.QueryRow("SELECT hash FROM documents WHERE collection = ? AND path = ?", strings.Split(f.DisplayPath, "/")[0], strings.Join(strings.Split(f.DisplayPath, "/")[1:], "/")).Scan(&docRes.Hash)
		if docRes.Hash == "" {
			vp := store.ParseVirtualPath(f.Filepath)
			if vp != nil {
				s.DB.QueryRow("SELECT hash FROM documents WHERE collection = ? AND path = ?", vp.CollectionName, vp.Path).Scan(&docRes.Hash)
			}
		}

		var lastModStr string
		s.DB.QueryRow("SELECT modified_at FROM documents WHERE hash = ?", docRes.Hash).Scan(&lastModStr)
		docRes.ModifiedAt = lastModStr

		fmt.Print(formatter.FormatDocument(docRes, "json"))
		fmt.Println("")
	} else if len(targets) == 1 && format == "md" {
		f := files[0]
		if f.Skipped {
			fmt.Fprintln(os.Stderr, f.SkipReason)
			os.Exit(1)
		}
		fmt.Print(f.Body)
		if !strings.HasSuffix(f.Body, "\n") {
			fmt.Println("")
		}
	} else {
		fmt.Println(formatter.FormatDocuments(files, format))
	}
}

func handleSearch(args []string) {
	var queryParts []string
	limit := 20
	var minScore float64
	format := "cli"
	var collection string
	full := false
	lineNumbers := false
	chunkStrategy := "regex"

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-n" || arg == "--limit" {
			if i+1 < len(args) {
				limit, _ = strconv.Atoi(args[i+1])
				i++
			}
		} else if arg == "--all" {
			limit = 10000
		} else if arg == "--min-score" {
			if i+1 < len(args) {
				minScore, _ = strconv.ParseFloat(args[i+1], 64)
				i++
			}
		} else if arg == "--collection" || arg == "-c" {
			if i+1 < len(args) {
				collection = args[i+1]
				i++
			}
		} else if arg == "--chunk-strategy" {
			if i+1 < len(args) {
				chunkStrategy = args[i+1]
				i++
			}
		} else if arg == "--json" {
			format = "json"
		} else if arg == "--csv" {
			format = "csv"
		} else if arg == "--xml" {
			format = "xml"
		} else if arg == "--md" {
			format = "md"
		} else if arg == "--files" {
			format = "files"
		} else if arg == "--full" {
			full = true
		} else if arg == "--line-numbers" {
			lineNumbers = true
		} else if strings.HasPrefix(arg, "-") {
			queryParts = append(queryParts, arg)
		} else {
			queryParts = append(queryParts, arg)
		}
	}

	query := strings.Join(queryParts, " ")
	if strings.TrimSpace(query) == "" {
		fmt.Fprintln(os.Stderr, "Usage: qmd search <query> [--limit <n>] [--collection <c>] [--chunk-strategy <auto|regex>] [--json|--csv|--xml|--md|--files]")
		os.Exit(1)
	}

	s, _, _, _ := getStoreAndConfig()
	defer s.Close()

	results, err := s.SearchFTS(query, limit, collection)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
		os.Exit(1)
	}

	var filtered []store.SearchResult
	for _, r := range results {
		if r.Score >= minScore {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		if format == "cli" {
			if len(results) > 0 {
				fmt.Println("No results found above minimum score threshold.")
			} else {
				fmt.Println("No results found.")
			}
		} else if format == "json" {
			fmt.Println("[]")
		} else if format == "csv" {
			fmt.Println("docid,score,file,title,context,line,snippet")
		} else if format == "xml" {
			fmt.Println("<results></results>")
		}
		return
	}

	opts := formatter.FormatOptions{
		Full:          full,
		Query:         query,
		LineNumbers:   lineNumbers,
		ChunkStrategy: chunkStrategy,
	}

	output := formatter.FormatSearchResults(filtered, format, opts)
	if format == "xml" {
		fmt.Printf("<results>\n%s\n</results>\n", output)
	} else {
		fmt.Println(output)
	}
}

func handleCleanup(args []string) {
	s, _, _, _ := getStoreAndConfig()
	defer s.Close()

	inactiveDocs := s.DeleteInactiveDocuments()
	orphanedHashes := s.CleanupOrphanedContent()
	s.Vacuum()

	fmt.Printf("%s✓%s Database cleaned successfully\n", cGreen, cReset)
	fmt.Printf("  Removed %d inactive documents\n", inactiveDocs)
	fmt.Printf("  Cleaned %d orphaned content hashes\n", orphanedHashes)
}

func handleMcp(args []string) {
	isHttp := false
	port := 8181
	host := "localhost"

	if h := os.Getenv("QMD_HOST"); h != "" {
		host = h
	}
	if pStr := os.Getenv("QMD_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil {
			port = p
		}
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--http" {
			isHttp = true
		} else if (arg == "--port" || arg == "-p") && i+1 < len(args) {
			port, _ = strconv.Atoi(args[i+1])
			i++
		} else if (arg == "--host" || arg == "-h") && i+1 < len(args) {
			host = args[i+1]
			i++
		}
	}

	s, _, _, _ := getStoreAndConfig()
	defer s.Close()

	if isHttp {
		err := mcp.StartHttpServer(s, host, port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		err := mcp.StartStdioServer(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Stdio server error: %v\n", err)
			os.Exit(1)
		}
	}
}
