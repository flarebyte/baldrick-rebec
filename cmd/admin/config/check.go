package configcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	cfgpkg "github.com/flarebyte/baldrick-rebec/internal/config"
	pgdao "github.com/flarebyte/baldrick-rebec/internal/dao/postgres"
	"github.com/spf13/cobra"
)

var (
	flagPasswords bool
	flagVerify    bool
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check configuration and report issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfgpkg.Load()
		if err != nil {
			return err
		}
		var problems []string

		if cfg.Server.Port <= 0 {
			problems = append(problems, "server.port must be > 0")
		}
		if cfg.Postgres.Host == "" {
			problems = append(problems, "postgres.host is required")
		}
		if cfg.Postgres.Port <= 0 {
			problems = append(problems, "postgres.port must be > 0")
		}
		if cfg.Postgres.DBName == "" {
			problems = append(problems, "postgres.dbname is required")
		}

		if flagPasswords {
			// Only show presence/absence, not values
			fmt.Fprintln(os.Stderr, "Password fields status (set=non-empty):")
			adminUser := cfg.Postgres.Admin.User
			if adminUser == "" {
				adminUser = "<unset>"
			}
			fmt.Fprintf(os.Stderr, "- postgres.admin.user: %s\n", adminUser)
			fmt.Fprintf(os.Stderr, "- postgres.admin.password: %v\n", cfg.Postgres.Admin.Password != "")
			fmt.Fprintf(os.Stderr, "- postgres.app.password: %v\n", cfg.Postgres.App.Password != "")
			// Legacy removed; no additional checks
		}

		if flagVerify {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			appdb, err := pgdao.OpenApp(ctx, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "verify: cannot connect with app role: %v\n", err)
			} else {
				defer appdb.Close()
				var exists, super, canLogin bool
				_ = appdb.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)`, cfg.Postgres.Admin.User).Scan(&exists)
				if exists {
					_ = appdb.QueryRow(ctx, `SELECT rolsuper, rolcanlogin FROM pg_roles WHERE rolname=$1`, cfg.Postgres.Admin.User).Scan(&super, &canLogin)
					fmt.Fprintf(os.Stderr, "verify: admin role %q: exists=%v superuser=%v can_login=%v\n", cfg.Postgres.Admin.User, exists, super, canLogin)
				} else {
					fmt.Fprintf(os.Stderr, "verify: admin role %q: not found. Superuser candidates: ", cfg.Postgres.Admin.User)
					rows, err := appdb.Query(ctx, `SELECT rolname FROM pg_roles WHERE rolsuper AND rolcanlogin ORDER BY rolname LIMIT 5`)
					if err == nil {
						first := true
						for rows.Next() {
							var rn string
							_ = rows.Scan(&rn)
							if !first {
								fmt.Fprint(os.Stderr, ", ")
							}
							fmt.Fprint(os.Stderr, rn)
							first = false
						}
						rows.Close()
					}
					fmt.Fprintln(os.Stderr)
				}
			}
		}

		if len(problems) > 0 {
			fmt.Fprintln(os.Stderr, "Configuration issues:")
			for _, p := range problems {
				fmt.Fprintf(os.Stderr, "- %s\n", p)
			}
			return errors.New(strings.Join(problems, "; "))
		}
		fmt.Fprintln(os.Stderr, "Configuration looks valid.")
		return nil
	},
}

func init() {
	checkCmd.Flags().BoolVar(&flagPasswords, "passwords", false, "Report which password fields are set (non-empty)")
	checkCmd.Flags().BoolVar(&flagVerify, "verify", false, "Verify configured admin role exists and is a superuser (uses app connection)")
}
