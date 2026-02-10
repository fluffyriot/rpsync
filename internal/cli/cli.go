// SPDX-License-Identifier: AGPL-3.0-only
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"syscall"

	"github.com/fluffyriot/rpsync/internal/authhelp"
	"github.com/fluffyriot/rpsync/internal/database"
	"golang.org/x/term"
)

func HandleResetPassword(dbQueries *database.Queries, username string) {
	ctx := context.Background()

	if username == "" {
		log.Fatal("--username is required")
	}

	user, err := dbQueries.GetUserByUsername(ctx, username)
	if err != nil {
		log.Fatalf("User '%s' not found: %v", username, err)
	}

	fmt.Printf("Enter new password for '%s': ", username)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("\nFailed to read password: %v", err)
	}
	fmt.Println()

	password := string(bytePassword)
	if err := authhelp.ValidatePasswordStrength(password); err != nil {
		log.Fatalf("Password is too weak: %v", err)
	}

	hash, err := authhelp.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	_, err = dbQueries.UpdateUserPassword(ctx, database.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: sql.NullString{String: hash, Valid: true},
	})
	if err != nil {
		log.Fatalf("Failed to update password: %v", err)
	}

	fmt.Println("Password updated successfully.")
}

func HandleReset2FA(dbQueries *database.Queries, username string) {
	ctx := context.Background()

	if username == "" {
		log.Fatal("--username is required")
	}

	user, err := dbQueries.GetUserByUsername(ctx, username)
	if err != nil {
		log.Fatalf("User '%s' not found: %v", username, err)
	}

	fmt.Printf("Resetting 2FA for user '%s'...\n", username)

	_, err = dbQueries.UpdateUserTOTP(ctx, database.UpdateUserTOTPParams{
		ID:          user.ID,
		TotpSecret:  sql.NullString{Valid: false},
		TotpEnabled: sql.NullBool{Valid: true, Bool: false},
	})
	if err != nil {
		log.Fatalf("Failed to reset 2FA: %v", err)
	}

	fmt.Printf("2FA successfully disabled for user '%s'\n", username)
}
