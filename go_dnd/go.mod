module github.com/humoroushorse/go_dnd

go 1.24.0

require (
	github.com/google/uuid v1.5.0
	github.com/humoroushorse/go_auth/pkg/auth v0.0.0-00010101000000-000000000000
	github.com/leanovate/gopter v0.2.9
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/golang-jwt/jwt/v5 v5.2.0 // indirect
	github.com/golang-migrate/migrate/v4 v4.17.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/lib/pq v1.10.9 // indirect
	go.uber.org/atomic v1.7.0 // indirect
)

// Local development: use local go_auth package
replace github.com/humoroushorse/go_auth/pkg/auth => ../go_auth/pkg/auth
