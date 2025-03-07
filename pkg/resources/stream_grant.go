package resources

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/helpers"
	"github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/snowflake"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var validStreamPrivileges = NewPrivilegeSet(
	privilegeOwnership,
	privilegeSelect,
)

var streamGrantSchema = map[string]*schema.Schema{
	"database_name": {
		Type:        schema.TypeString,
		Required:    true,
		Description: "The name of the database containing the current or future streams on which to grant privileges.",
		ForceNew:    true,
	},
	"enable_multiple_grants": {
		Type:        schema.TypeBool,
		Optional:    true,
		Description: "When this is set to true, multiple grants of the same type can be created. This will cause Terraform to not revoke grants applied to roles and objects outside Terraform.",
		Default:     false,
		ForceNew:    true,
	},
	"on_future": {
		Type:        schema.TypeBool,
		Optional:    true,
		Description: "When this is set to true and a schema_name is provided, apply this grant on all future streams in the given schema. When this is true and no schema_name is provided apply this grant on all future streams in the given database. The stream_name field must be unset in order to use on_future.",
		Default:     false,
		ForceNew:    true,
	},
	"privilege": {
		Type:         schema.TypeString,
		Optional:     true,
		Description:  "The privilege to grant on the current or future stream.",
		Default:      "SELECT",
		ValidateFunc: validation.StringInSlice(validStreamPrivileges.ToList(), true),
		ForceNew:     true,
	},
	"roles": {
		Type:        schema.TypeSet,
		Required:    true,
		Elem:        &schema.Schema{Type: schema.TypeString},
		Description: "Grants privilege to these roles.",
	},
	"schema_name": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The name of the schema containing the current or future streams on which to grant privileges.",
		ForceNew:    true,
	},
	"stream_name": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The name of the stream on which to grant privileges immediately (only valid if on_future is false).",
		ForceNew:    true,
	},
	"with_grant_option": {
		Type:        schema.TypeBool,
		Optional:    true,
		Description: "When this is set to true, allows the recipient role to grant the privileges to other roles.",
		Default:     false,
		ForceNew:    true,
	},
}

// StreamGrant returns a pointer to the resource representing a stream grant.
func StreamGrant() *TerraformGrantResource {
	return &TerraformGrantResource{
		Resource: &schema.Resource{
			Create: CreateStreamGrant,
			Read:   ReadStreamGrant,
			Delete: DeleteStreamGrant,
			Update: UpdateStreamGrant,

			Schema: streamGrantSchema,
			Importer: &schema.ResourceImporter{
				StateContext: schema.ImportStatePassthroughContext,
			},
		},
		ValidPrivs: validStreamPrivileges,
	}
}

// CreateStreamGrant implements schema.CreateFunc.
func CreateStreamGrant(d *schema.ResourceData, meta interface{}) error {
	var streamName string
	if name, ok := d.GetOk("stream_name"); ok {
		streamName = name.(string)
	}
	databaseName := d.Get("database_name").(string)
	schemaName := d.Get("schema_name").(string)
	privilege := d.Get("privilege").(string)
	onFuture := d.Get("on_future").(bool)
	withGrantOption := d.Get("with_grant_option").(bool)
	roles := expandStringList(d.Get("roles").(*schema.Set).List())

	if (streamName == "") && !onFuture {
		return errors.New("stream_name must be set unless on_future is true")
	}
	if (streamName != "") && onFuture {
		return errors.New("stream_name must be empty if on_future is true")
	}
	if (schemaName == "") && !onFuture {
		return errors.New("schema_name must be set unless on_future is true")
	}

	var builder snowflake.GrantBuilder
	if onFuture {
		builder = snowflake.FutureStreamGrant(databaseName, schemaName)
	} else {
		builder = snowflake.StreamGrant(databaseName, schemaName, streamName)
	}

	if err := createGenericGrant(d, meta, builder); err != nil {
		return err
	}

	grantID := NewStreamGrantID(databaseName, schemaName, streamName, privilege, roles, withGrantOption)
	d.SetId(grantID.String())

	return ReadStreamGrant(d, meta)
}

// ReadStreamGrant implements schema.ReadFunc.
func ReadStreamGrant(d *schema.ResourceData, meta interface{}) error {
	grantID, err := parseStreamGrantID(d.Id())
	if err != nil {
		return err
	}

	if err := d.Set("database_name", grantID.DatabaseName); err != nil {
		return err
	}

	if err := d.Set("schema_name", grantID.SchemaName); err != nil {
		return err
	}
	onFuture := false
	if grantID.ObjectName == "" {
		onFuture = true
	}

	if err := d.Set("stream_name", grantID.ObjectName); err != nil {
		return err
	}

	if err := d.Set("on_future", onFuture); err != nil {
		return err
	}

	if err := d.Set("privilege", grantID.Privilege); err != nil {
		return err
	}

	if err := d.Set("with_grant_option", grantID.WithGrantOption); err != nil {
		return err
	}

	var builder snowflake.GrantBuilder
	if onFuture {
		builder = snowflake.FutureStreamGrant(grantID.DatabaseName, grantID.SchemaName)
	} else {
		builder = snowflake.StreamGrant(grantID.DatabaseName, grantID.SchemaName, grantID.ObjectName)
	}

	return readGenericGrant(d, meta, streamGrantSchema, builder, onFuture, validStreamPrivileges)
}

// DeleteStreamGrant implements schema.DeleteFunc.
func DeleteStreamGrant(d *schema.ResourceData, meta interface{}) error {
	grantID, err := parseStreamGrantID(d.Id())
	if err != nil {
		return err
	}

	onFuture := (grantID.ObjectName == "")

	var builder snowflake.GrantBuilder
	if onFuture {
		builder = snowflake.FutureStreamGrant(grantID.DatabaseName, grantID.SchemaName)
	} else {
		builder = snowflake.StreamGrant(grantID.DatabaseName, grantID.SchemaName, grantID.ObjectName)
	}
	return deleteGenericGrant(d, meta, builder)
}

// UpdateStreamGrant implements schema.UpdateFunc.
func UpdateStreamGrant(d *schema.ResourceData, meta interface{}) error {
	// for now the only thing we can update are roles or shares
	// if nothing changed, nothing to update and we're done
	if !d.HasChanges("roles") {
		return nil
	}

	rolesToAdd := []string{}
	rolesToRevoke := []string{}

	if d.HasChange("roles") {
		rolesToAdd, rolesToRevoke = changeDiff(d, "roles")
	}

	grantID, err := parseStreamGrantID(d.Id())
	if err != nil {
		return err
	}

	onFuture := (grantID.ObjectName == "")

	var builder snowflake.GrantBuilder
	if onFuture {
		builder = snowflake.FutureStreamGrant(grantID.DatabaseName, grantID.SchemaName)
	} else {
		builder = snowflake.StreamGrant(grantID.DatabaseName, grantID.SchemaName, grantID.ObjectName)
	}

	// first revoke
	if err := deleteGenericGrantRolesAndShares(
		meta, builder, grantID.Privilege, rolesToRevoke, []string{},
	); err != nil {
		return err
	}
	// then add
	if err := createGenericGrantRolesAndShares(
		meta, builder, grantID.Privilege, grantID.WithGrantOption, rolesToAdd, []string{},
	); err != nil {
		return err
	}

	// Done, refresh state
	return ReadStreamGrant(d, meta)
}

type StreamGrantID struct {
	DatabaseName    string
	SchemaName      string
	ObjectName      string
	Privilege       string
	Roles           []string
	WithGrantOption bool
}

func NewStreamGrantID(databaseName string, schemaName, objectName, privilege string, roles []string, withGrantOption bool) *StreamGrantID {
	return &StreamGrantID{
		DatabaseName:    databaseName,
		SchemaName:      schemaName,
		ObjectName:      objectName,
		Privilege:       privilege,
		Roles:           roles,
		WithGrantOption: withGrantOption,
	}
}

func (v *StreamGrantID) String() string {
	roles := strings.Join(v.Roles, ",")
	return fmt.Sprintf("%v❄️%v❄️%v❄️%v❄️%v❄️%v", v.DatabaseName, v.SchemaName, v.ObjectName, v.Privilege, v.WithGrantOption, roles)
}

func parseStreamGrantID(s string) (*StreamGrantID, error) {
	// is this an old ID format?
	if !strings.Contains(s, "❄️") {
		idParts := strings.Split(s, "|")
		return &StreamGrantID{
			DatabaseName:    idParts[0],
			SchemaName:      idParts[1],
			ObjectName:      idParts[2],
			Privilege:       idParts[3],
			Roles:           []string{},
			WithGrantOption: idParts[4] == "true",
		}, nil
	}
	idParts := strings.Split(s, "❄️")
	if len(idParts) != 6 {
		return nil, fmt.Errorf("unexpected number of ID parts (%d), expected 6", len(idParts))
	}
	return &StreamGrantID{
		DatabaseName:    idParts[0],
		SchemaName:      idParts[1],
		ObjectName:      idParts[2],
		Privilege:       idParts[3],
		WithGrantOption: idParts[4] == "true",
		Roles:           helpers.SplitStringToSlice(idParts[5], ","),
	}, nil
}
