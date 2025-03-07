---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "snowflake_tag_grant Resource - terraform-provider-snowflake"
subcategory: ""
description: |-
  
---

# snowflake_tag_grant (Resource)



## Example Usage

```terraform
resource "snowflake_tag_grant" "example" {
  database_name = "database"
  schema_name   = "schema"
  tag_name      = "tag"
  roles         = ["TEST_ROLE"]
  privilege     = "OWNERSHIP"

}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `database_name` (String) The name of the database containing the tag on which to grant privileges.
- `schema_name` (String) The name of the schema containing the tag on which to grant privileges.
- `tag_name` (String) The name of the tag on which to grant privileges.

### Optional

- `enable_multiple_grants` (Boolean) When this is set to true, multiple grants of the same type can be created. This will cause Terraform to not revoke grants applied to roles and objects outside Terraform.
- `privilege` (String) The privilege to grant on the tag.
- `roles` (Set of String) Grants privilege to these roles.
- `with_grant_option` (Boolean) When this is set to true, allows the recipient role to grant the privileges to other roles.

### Read-Only

- `id` (String) The ID of this resource.

## Import

Import is supported using the following syntax:

```shell
# format is database_name ❄️ schema_name ❄️ tag_name ❄️ privilege ❄️ with_grant_option ❄️ roles
terraform import snowflake_tag_grant.example 'MY_DATABASE❄️MY_SCHEMA❄️MY_OBJECT❄️APPLY❄️false❄️role1,role2'
```
