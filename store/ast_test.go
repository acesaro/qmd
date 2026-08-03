package store

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want SupportedLanguage
	}{
		{"src/auth.ts", LangTypeScript},
		{"src/auth.mts", LangTypeScript},
		{"src/auth.cts", LangTypeScript},
		{"src/App.tsx", LangTSX},
		{"src/util.js", LangJavaScript},
		{"src/util.mjs", LangJavaScript},
		{"src/util.cjs", LangJavaScript},
		{"src/App.jsx", LangTSX},
		{"src/auth.py", LangPython},
		{"src/auth.go", LangGo},
		{"src/auth.rs", LangRust},
		{"docs/README.md", LangNone},
		{"data/file.csv", LangNone},
		{"src/Auth.TS", LangTypeScript},
		{"qmd://myproject/src/auth.ts", LangTypeScript},
	}

	for _, tt := range tests {
		got := detectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestGetASTBreakPointsTypeScript(t *testing.T) {
	tsSample := `import { Database } from './db';
import type { User } from './types';

interface AuthConfig {
  secret: string;
  ttl: number;
}

type UserId = string;

export class AuthService {
  constructor(private db: Database) {}

  async authenticate(user: User, token: string): Promise<boolean> {
    const session = await this.db.findSession(token);
    return session?.userId === user.id;
  }

  validateToken(token: string): boolean {
    return token.length === 64;
  }
}

export function hashPassword(password: string): string {
  return crypto.createHash('sha256').update(password).digest('hex');
}
`

	points := GetASTBreakPoints(tsSample, "src/auth.ts")
	if len(points) == 0 {
		t.Fatalf("expected break points, got 0")
	}

	// Verify types
	var hasImport, hasIface, hasType, hasClass, hasMethod, hasFunc bool
	for _, p := range points {
		switch p.Type {
		case "ast:import":
			hasImport = true
			if p.Score != 60 {
				t.Errorf("import score %d, want 60", p.Score)
			}
		case "ast:iface":
			hasIface = true
			if p.Score != 100 {
				t.Errorf("iface score %d, want 100", p.Score)
			}
		case "ast:type":
			hasType = true
			if p.Score != 80 {
				t.Errorf("type score %d, want 80", p.Score)
			}
		case "ast:class":
			hasClass = true
			if p.Score != 100 {
				t.Errorf("class score %d, want 100", p.Score)
			}
		case "ast:method":
			hasMethod = true
			if p.Score != 90 {
				t.Errorf("method score %d, want 90", p.Score)
			}
		case "ast:func":
			hasFunc = true
			if p.Score != 90 {
				t.Errorf("func score %d, want 90", p.Score)
			}
		}
	}

	if !hasImport {
		t.Error("missing import point")
	}
	if !hasIface {
		t.Error("missing interface point")
	}
	if !hasType {
		t.Error("missing type point")
	}
	if !hasClass {
		t.Error("missing class point")
	}
	if !hasMethod {
		t.Error("missing method point")
	}
	if !hasFunc {
		t.Error("missing func point")
	}

	// Verify order
	for i := 1; i < len(points); i++ {
		if points[i].Pos < points[i-1].Pos {
			t.Errorf("points out of order: pos %d < pos %d", points[i].Pos, points[i-1].Pos)
		}
	}
}

func TestGetASTBreakPointsPython(t *testing.T) {
	pySample := `import os
from typing import Optional

class AuthService:
    def __init__(self, db):
        self.db = db

    async def authenticate(self, user, token):
        session = await self.db.find(token)
        return session.user_id == user.id

    def validate_token(self, token):
        return len(token) == 64

def hash_password(password: str) -> str:
    return hashlib.sha256(password.encode()).hexdigest()

@decorator
def decorated_func():
    pass
`

	points := GetASTBreakPoints(pySample, "auth.py")
	var hasImport, hasClass, hasFunc, hasDecorated bool
	for _, p := range points {
		switch p.Type {
		case "ast:import":
			hasImport = true
		case "ast:class":
			hasClass = true
		case "ast:func":
			hasFunc = true
		case "ast:decorated":
			hasDecorated = true
		}
	}

	if !hasImport {
		t.Error("missing python import point")
	}
	if !hasClass {
		t.Error("missing python class point")
	}
	if !hasFunc {
		t.Error("missing python func point")
	}
	if !hasDecorated {
		t.Error("missing python decorated point")
	}
}

func TestGetASTBreakPointsGo(t *testing.T) {
	goSample := `package main

import "fmt"

type AuthService struct {
    db *Database
}

func (s *AuthService) Authenticate(user User) bool {
    return true
}

func HashPassword(password string) string {
    return "hash"
}
`

	points := GetASTBreakPoints(goSample, "auth.go")
	var hasImport, hasType, hasMethod, hasFunc bool
	for _, p := range points {
		switch p.Type {
		case "ast:import":
			hasImport = true
		case "ast:type":
			hasType = true
		case "ast:method":
			hasMethod = true
		case "ast:func":
			hasFunc = true
		}
	}

	if !hasImport {
		t.Error("missing go import point")
	}
	if !hasType {
		t.Error("missing go type point")
	}
	if !hasMethod {
		t.Error("missing go method point")
	}
	if !hasFunc {
		t.Error("missing go func point")
	}
}

func TestGetASTBreakPointsRust(t *testing.T) {
	rsSample := `use std::collections::HashMap;

struct AuthService {
    db: Database,
}

impl AuthService {
    fn authenticate(&self, user: &User) -> bool {
        true
    }
}

trait Authenticatable {
    fn validate(&self) -> bool;
}

enum Role {
    Admin,
    User,
}

fn hash_password(password: &str) -> String {
    String::new()
}
`

	points := GetASTBreakPoints(rsSample, "auth.rs")
	var hasImport, hasStruct, hasImpl, hasTrait, hasEnum, hasFunc bool
	for _, p := range points {
		switch p.Type {
		case "ast:import":
			hasImport = true
		case "ast:struct":
			hasStruct = true
		case "ast:impl":
			hasImpl = true
		case "ast:trait":
			hasTrait = true
		case "ast:enum":
			hasEnum = true
		case "ast:func":
			hasFunc = true
		}
	}

	if !hasImport {
		t.Error("missing rust import point")
	}
	if !hasStruct {
		t.Error("missing rust struct point")
	}
	if !hasImpl {
		t.Error("missing rust impl point")
	}
	if !hasTrait {
		t.Error("missing rust trait point")
	}
	if !hasEnum {
		t.Error("missing rust enum point")
	}
	if !hasFunc {
		t.Error("missing rust func point")
	}
}

func TestGetASTBreakPointsSql(t *testing.T) {
	sqlSample := `CREATE DATABASE mydb;
CREATE SCHEMA myschema;
CREATE TABLE users (id INT, name TEXT);
CREATE VIEW user_view AS SELECT * FROM users;
CREATE OR REPLACE FUNCTION get_user(id INT) RETURNS TEXT AS $$ SELECT name FROM users WHERE id = id $$ LANGUAGE SQL;
CREATE TYPE user_status AS ENUM ('active', 'inactive');
`
	points := GetASTBreakPoints(sqlSample, "schema.sql")
	var hasStruct, hasFunc, hasType bool
	for _, p := range points {
		switch p.Type {
		case "ast:struct":
			hasStruct = true
		case "ast:func":
			hasFunc = true
		case "ast:type":
			hasType = true
		}
	}

	if !hasStruct {
		t.Error("missing sql struct (table/view/schema/db) point")
	}
	if !hasFunc {
		t.Error("missing sql func point")
	}
	if !hasType {
		t.Error("missing sql type point")
	}
}

func TestGetASTBreakPointsKql(t *testing.T) {
	kqlSample := `let min_fee = 10;
let get_transactions = (t:string) {
    Transactions | where Type == t | where Fee > min_fee
};
get_transactions("wire")
`
	points := GetASTBreakPoints(kqlSample, "query.kql")
	var hasFunc bool
	for _, p := range points {
		if p.Type == "ast:func" {
			hasFunc = true
		}
	}

	if !hasFunc {
		t.Error("missing kql func (let binding) point")
	}
}

func TestGetASTBreakPointsLua(t *testing.T) {
	luaSample := `local util = require("util")
local M = {}

local function private_func()
    print("private")
end

function M.public_func(x)
    return x * 2
end

return M
`
	points := GetASTBreakPoints(luaSample, "module.lua")
	var hasImport, hasStruct, hasFunc bool
	for _, p := range points {
		switch p.Type {
		case "ast:import":
			hasImport = true
		case "ast:struct":
			hasStruct = true
		case "ast:func":
			hasFunc = true
		}
	}

	if !hasImport {
		t.Error("missing lua import point")
	}
	if !hasStruct {
		t.Error("missing lua struct (module table) point")
	}
	if !hasFunc {
		t.Error("missing lua func point")
	}
}
