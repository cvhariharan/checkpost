---
title: Access
weight: 30
---

# Access

Users, user groups, and API tokens control who can get into Checkpost and what they can do after they sign in.

## Users

The Users page creates, edits, disables, and deletes Checkpost users. A user may receive a built-in role directly or through a user group.

Checkpost always ensures the administrator configured by `app.admin_username` exists. Set a strong `app.admin_password`.

## OIDC

OIDC is enabled when issuer, client ID, and client secret are all set. Checkpost can restrict email domains, create users on first login, and read memberships from a configurable groups claim.

`default_role` grants every new OIDC user a global role. Leave it empty when an administrator should approve access first.

The built-in roles are:

| Role | Access |
| --- | --- |
| Admin | Full access, including users, user groups, role bindings, and settings |
| Operator | Manage machines, groups, policies, schedules, YARA, inventory, and alerts |
| Analyst | Read data, run live queries, and start YARA scans |
| Viewer | Read-only access |

## User groups

User groups grant the same role to several users. Add members manually or set an OIDC group claim value to map sign-ins to the group.

## API tokens

API tokens authenticate `checkpost apply` and direct API requests. Tokens have the same permissions as the issuing user.
