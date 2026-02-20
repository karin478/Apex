# Phase 20: Hypothesis Board — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Structured hypothesis tracking with propose/challenge/confirm/reject lifecycle and evidence scoring.

**Architecture:** New `internal/hypothesis` package with `Board`, `Hypothesis`, `Evidence` types. JSON persistence per session. CLI subcommands under `apex hypothesis`.

**Tech Stack:** Go, Cobra CLI, Testify, encoding/json

---

## Task 1: Hypothesis Core — Board + CRUD Operations

**Files:**
- Create: `internal/hypothesis/board.go`
- Create: `internal/hypothesis/board_test.go`

**Types:** Board, Hypothesis, Evidence, Status (PROPOSED/CHALLENGED/CONFIRMED/REJECTED)

**Methods:** NewBoard, Propose, Challenge, Confirm, Reject, Get, List, Score, Save, LoadBoard

**Tests (8):**
- TestNewBoard
- TestPropose — adds hypothesis, returns ID, status=PROPOSED
- TestChallenge — changes status to CHALLENGED, adds evidence
- TestConfirm — changes status to CONFIRMED, adds evidence
- TestReject — changes status to REJECTED
- TestGetNotFound — returns error for nonexistent ID
- TestScore — avg confidence of evidence
- TestSaveAndLoad — round-trip persistence

**Commit:** `feat(hypothesis): add Board with propose/challenge/confirm/reject lifecycle`

---

## Task 2: CLI Command — `apex hypothesis`

**Files:**
- Create: `cmd/apex/hypothesis.go`
- Modify: `cmd/apex/main.go` (register hypothesisCmd)

**Subcommands:**
- `apex hypothesis list` — list hypotheses
- `apex hypothesis propose "statement"` — add new
- `apex hypothesis confirm <id> "evidence"` — confirm
- `apex hypothesis reject <id> "reason"` — reject

**Commit:** `feat(cli): add apex hypothesis command with subcommands`

---

## Task 3: E2E Tests

**Files:**
- Create: `e2e/hypothesis_test.go`

**Tests (3):**
- TestHypothesisListEmpty — list on fresh env
- TestHypothesisProposeAndList — propose then list
- TestHypothesisConfirm — propose then confirm

**Commit:** `test(e2e): add hypothesis board E2E tests`

---

## Task 4: Update PROGRESS.md

**Commit:** `docs: mark Phase 20 Hypothesis Board as complete`
