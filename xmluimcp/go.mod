module github.com/xmlui-org/mcpsvr/xmluimcp

go 1.25.3

require (
	github.com/mark3labs/mcp-go v0.43.0
	github.com/mikeschinkel/go-cfgstore v0.0.0-20251108093120-b95cd78bd5f3
	github.com/mikeschinkel/go-cliutil v0.0.0-20251108022717-47edd25bdf26
	github.com/mikeschinkel/go-doterr v0.0.0-20251103083058-e8b2711e9cf5
	github.com/mikeschinkel/go-dt v0.0.0-20251107040413-53a1559d69c5
	github.com/mikeschinkel/go-dt/appinfo v0.0.0-20251107040413-53a1559d69c5
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/invopop/jsonschema v0.13.0 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mikeschinkel/go-dt/de v0.0.0-20251107040413-53a1559d69c5 // indirect
	github.com/mikeschinkel/go-dt/dtx v0.0.0-20251107040413-53a1559d69c5 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/mikeschinkel/go-cfgstore => ../../../go-pkgs/go-cfgstore
	github.com/mikeschinkel/go-cliutil => ../../../go-pkgs/go-cliutil
	github.com/mikeschinkel/go-doterr => ../../../go-pkgs/go-doterr
	github.com/mikeschinkel/go-dt => ../../../go-pkgs/go-dt
	github.com/mikeschinkel/go-dt/appinfo => ../../../go-pkgs/go-dt/appinfo
	github.com/mikeschinkel/go-dt/de => ../../../go-pkgs/go-dt/de
	github.com/mikeschinkel/go-dt/dtx => ../../../go-pkgs/go-dt/dtx
	github.com/mikeschinkel/go-fsfix => ../../../go-pkgs/go-fsfix
	github.com/mikeschinkel/go-jsontest => ../../../go-pkgs/go-jsontest
	github.com/mikeschinkel/go-jsonxtractr => ../../../go-pkgs/go-jsonxtractr
	github.com/mikeschinkel/go-rfc9457 => ../../../go-pkgs/go-rfc9457
	github.com/mikeschinkel/go-testutil => ../../../go-pkgs/go-testutil
)
