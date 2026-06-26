package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type rootCreds struct {
	Username string
	Password string
	Host     string
	Port     string
}

// fetchRootCreds reads the KubeBlocks conn-credential Secret for the cluster.
func (c *kubeClient) fetchRootCreds(ctx context.Context, namespace string, r *dbRecord) (*rootCreds, error) {
	secretName := r.ResourceName + "-conn-credential"
	var secret kubeSecret
	if err := c.doJSON(ctx, "GET", fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, secretName), nil, &secret); err != nil {
		return nil, err
	}
	creds := &rootCreds{
		Username: decodeBase64(secret.Data["username"]),
		Password: decodeBase64(secret.Data["password"]),
		Host:     decodeBase64(secret.Data["host"]),
		Port:     decodeBase64(secret.Data["port"]),
	}
	if creds.Host == "" {
		creds.Host = fmt.Sprintf("%s-%s.%s.svc.cluster.local", r.ResourceName, componentNames[r.Type], namespace)
	}
	if creds.Port == "" {
		creds.Port = dbPorts[r.Type]
	}
	return creds, nil
}

// openSQLConn opens a *sql.DB to the instance using root credentials. The
// caller must Close() it. dbName may be empty (then connects to no specific
// database, useful for CREATE DATABASE).
func openSQLConn(creds *rootCreds, dbType, dbName string) (*sql.DB, error) {
	switch dbType {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=5s",
			creds.Username, creds.Password, creds.Host, creds.Port, dbName)
		return sql.Open("mysql", dsn)
	case "postgres":
		target := dbName
		if target == "" {
			target = "postgres"
		}
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable connect_timeout=5",
			creds.Host, creds.Port, creds.Username, creds.Password, target)
		return sql.Open("pgx", dsn)
	default:
		return nil, fmt.Errorf("schema management is not supported for db type %q", dbType)
	}
}

// listSchemas returns user-visible schemas (system DBs filtered out).
func (c *kubeClient) listSchemas(ctx context.Context, namespace string, r *dbRecord) ([]string, error) {
	creds, err := c.fetchRootCreds(ctx, namespace, r)
	if err != nil {
		return nil, err
	}
	db, err := openSQLConn(creds, r.Type, "")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var (
		query    string
		excluded map[string]bool
	)
	switch r.Type {
	case "mysql":
		query = "SHOW DATABASES"
		excluded = map[string]bool{"information_schema": true, "mysql": true, "performance_schema": true, "sys": true}
	case "postgres":
		query = "SELECT datname FROM pg_database WHERE datistemplate = false"
		excluded = map[string]bool{"postgres": true}
	default:
		return nil, fmt.Errorf("unsupported db type %q", r.Type)
	}

	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if excluded[strings.ToLower(name)] {
			continue
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// createSchema creates a new logical database. charset only applies to MySQL.
func (c *kubeClient) createSchema(ctx context.Context, namespace string, r *dbRecord, schemaName, charset string) error {
	if err := validateIdentifier(schemaName); err != nil {
		return err
	}
	creds, err := c.fetchRootCreds(ctx, namespace, r)
	if err != nil {
		return err
	}
	db, err := openSQLConn(creds, r.Type, "")
	if err != nil {
		return err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var stmt string
	switch r.Type {
	case "mysql":
		stmt = fmt.Sprintf("CREATE DATABASE `%s`", schemaName)
		if cs := strings.TrimSpace(charset); cs != "" {
			if err := validateIdentifier(cs); err != nil {
				return err
			}
			stmt += fmt.Sprintf(" CHARACTER SET %s", cs)
		}
	case "postgres":
		stmt = fmt.Sprintf(`CREATE DATABASE "%s"`, schemaName)
	default:
		return fmt.Errorf("unsupported db type %q", r.Type)
	}

	if _, err := db.ExecContext(queryCtx, stmt); err != nil {
		return err
	}
	return nil
}

// deleteSchema drops a logical database.
func (c *kubeClient) deleteSchema(ctx context.Context, namespace string, r *dbRecord, schemaName string) error {
	if err := validateIdentifier(schemaName); err != nil {
		return err
	}
	creds, err := c.fetchRootCreds(ctx, namespace, r)
	if err != nil {
		return err
	}
	db, err := openSQLConn(creds, r.Type, "")
	if err != nil {
		return err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var stmt string
	switch r.Type {
	case "mysql":
		stmt = fmt.Sprintf("DROP DATABASE `%s`", schemaName)
	case "postgres":
		stmt = fmt.Sprintf(`DROP DATABASE "%s"`, schemaName)
	default:
		return fmt.Errorf("unsupported db type %q", r.Type)
	}

	if _, err := db.ExecContext(queryCtx, stmt); err != nil {
		return err
	}
	return nil
}

// validateIdentifier guards against SQL injection in identifiers since they
// cannot be parameterized. Allows [A-Za-z0-9_], 1-64 chars.
func validateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("%w: identifier is empty", errInvalidArgument)
	}
	if len(name) > 64 {
		return fmt.Errorf("%w: identifier too long", errInvalidArgument)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_':
			continue
		default:
			return fmt.Errorf("%w: identifier contains invalid character", errInvalidArgument)
		}
	}
	return nil
}
