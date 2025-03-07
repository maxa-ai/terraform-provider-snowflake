package resources_test

import (
	"database/sql"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/provider"
	"github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/resources"
	"github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/snowflake"
	"github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/testhelpers"
	. "github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/testhelpers"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"
)

func TestView(t *testing.T) {
	r := require.New(t)
	err := resources.View().InternalValidate(provider.Provider().Schema, true)
	r.NoError(err)
}

func TestViewCreate(t *testing.T) {
	r := require.New(t)

	in := map[string]interface{}{
		"name":      "good_name",
		"database":  "test_db",
		"schema":    "test_schema",
		"comment":   "great comment",
		"statement": "SELECT * FROM test_db.PUBLIC.GREAT_TABLE WHERE account_id = 'bobs-account-id'",
		"is_secure": true,
	}
	d := schema.TestResourceDataRaw(t, resources.View().Schema, in)
	r.NotNil(d)

	WithMockDb(t, func(db *sql.DB, mock sqlmock.Sqlmock) {
		mock.ExpectExec(
			`^CREATE SECURE VIEW "test_db"."test_schema"."good_name" COMMENT = 'great comment' AS SELECT \* FROM test_db.PUBLIC.GREAT_TABLE WHERE account_id = 'bobs-account-id'$`,
		).WillReturnResult(sqlmock.NewResult(1, 1))

		expectReadView(mock)
		err := resources.CreateView(d, db)
		r.NoError(err)
	})
}

func TestViewCreateOrReplace(t *testing.T) {
	r := require.New(t)

	in := map[string]interface{}{
		"name":       "good_name",
		"database":   "test_db",
		"schema":     "test_schema",
		"comment":    "great comment",
		"statement":  "SELECT * FROM test_db.PUBLIC.GREAT_TABLE WHERE account_id = 'bobs-account-id'",
		"is_secure":  true,
		"or_replace": true,
	}
	d := schema.TestResourceDataRaw(t, resources.View().Schema, in)
	r.NotNil(d)

	WithMockDb(t, func(db *sql.DB, mock sqlmock.Sqlmock) {
		mock.ExpectExec(
			`^CREATE OR REPLACE SECURE VIEW "test_db"."test_schema"."good_name" COMMENT = 'great comment' AS SELECT \* FROM test_db.PUBLIC.GREAT_TABLE WHERE account_id = 'bobs-account-id'$`,
		).WillReturnResult(sqlmock.NewResult(1, 1))

		expectReadView(mock)
		err := resources.CreateView(d, db)
		r.NoError(err)
	})
}

func TestViewCreateAmpersand(t *testing.T) {
	r := require.New(t)

	in := map[string]interface{}{
		"name":      "good_name",
		"database":  "test_db",
		"schema":    "test_schema",
		"comment":   "great comment",
		"statement": "SELECT * FROM test_db.PUBLIC.GREAT_TABLE WHERE account_id LIKE 'bob%'",
		"is_secure": true,
	}
	d := schema.TestResourceDataRaw(t, resources.View().Schema, in)
	r.NotNil(d)

	WithMockDb(t, func(db *sql.DB, mock sqlmock.Sqlmock) {
		mock.ExpectExec(
			`^CREATE SECURE VIEW "test_db"."test_schema"."good_name" COMMENT = 'great comment' AS SELECT \* FROM test_db.PUBLIC.GREAT_TABLE WHERE account_id LIKE 'bob%'$`,
		).WillReturnResult(sqlmock.NewResult(1, 1))

		expectReadView(mock)
		err := resources.CreateView(d, db)
		r.NoError(err)
	})
}

func expectReadView(mock sqlmock.Sqlmock) {
	rows := sqlmock.NewRows([]string{
		"created_on", "name", "reserved", "database_name", "schema_name", "owner", "comment", "text", "is_secure", "is_materialized",
	},
	).AddRow("2019-05-19 16:55:36.530 -0700", "good_name", "", "test_db", "test_schema", "admin", "great comment", "SELECT * FROM test_db.GREAT_SCHEMA.GREAT_TABLE WHERE account_id = 'bobs-account-id'", true, false)
	mock.ExpectQuery(`^SHOW VIEWS LIKE 'good_name' IN SCHEMA "test_db"."test_schema"$`).WillReturnRows(rows)
}

func TestDiffSuppressStatement(t *testing.T) {
	type args struct {
		k   string
		old string
		new string
		d   *schema.ResourceData
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"select", args{"", "select * from foo;", "select * from foo;", nil}, true},
		{"view 1", args{"", testhelpers.MustFixture(t, "view_1a.sql"), testhelpers.MustFixture(t, "view_1b.sql"), nil}, true},
		{"view 2", args{"", testhelpers.MustFixture(t, "view_2a.sql"), testhelpers.MustFixture(t, "view_2b.sql"), nil}, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := resources.DiffSuppressStatement(tt.args.k, tt.args.old, tt.args.new, tt.args.d); got != tt.want {
				t.Errorf("DiffSuppressStatement() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestViewRead(t *testing.T) {
	r := require.New(t)

	in := map[string]interface{}{
		"name":     "good_name",
		"database": "test_db",
		"schema":   "test_schema",
	}

	d := view(t, "test_db|test_schema|good_name", in)

	WithMockDb(t, func(db *sql.DB, mock sqlmock.Sqlmock) {
		// Test when resource is not found, checking if state will be empty
		r.NotEmpty(d.State())
		q := snowflake.NewViewBuilder("good_name").WithDB("test_db").WithSchema("test_schema").Show()
		fmt.Println(q)
		mock.ExpectQuery(q).WillReturnError(sql.ErrNoRows)
		err := resources.ReadView(d, db)
		r.Empty(d.State())
		r.Nil(err)
	})
}
