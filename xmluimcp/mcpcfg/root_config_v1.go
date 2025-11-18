package mcpcfg

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"

	"github.com/mikeschinkel/go-cfgstore"
	"github.com/mikeschinkel/go-dt/appinfo"
	"github.com/xmlui-org/mcpsvr/xmluimcp/common"
)

const (
	RootConfigV1Version = 1
	RootConfigV1Schema  = "https://xmlui.org/schemas/v1/mcp-server/root-schema.json"
)

var _ Config = (*RootConfigV1)(nil)
var _ cfgstore.RootConfig = (*RootConfigV1)(nil)

// RootConfigV1 represents the root configuration structure for XMLUI MCP server
type RootConfigV1 struct {
	rootConfigV1Base `json:",inline"`
}

func (c *RootConfigV1) RootConfig() {}

// Base struct with all fields
type rootConfigV1Base struct {
	Schema        string   `json:"$schema"`
	Version       int      `json:"version"`
	XMLUIDir      string   `json:"xmlui_dir"`
	ExampleRoot   string   `json:"example_root,omitempty"`
	ExampleDirs   []string `json:"example_dirs,omitempty"`
	HTTPMode      bool     `json:"http_mode,omitempty"`
	Port          string   `json:"port,omitempty"`
	AnalyticsFile string   `json:"analytics_file,omitempty"`
}

func NewRootConfigV1() (c *RootConfigV1) {
	return &RootConfigV1{
		rootConfigV1Base: rootConfigV1Base{
			Schema:  RootConfigV1Schema,
			Version: RootConfigV1Version,
		},
	}
}

func (c *RootConfigV1) Config() {}

func (c *RootConfigV1) Normalize(cfgstore.NormalizeArgs) (err error) {
	c.Schema = RootConfigV1Schema
	c.Version = RootConfigV1Version
	return err
}

func (c *RootConfigV1) String() string {
	return string(c.Bytes())
}

func (c *RootConfigV1) Bytes() []byte {
	b, err := jsonv2.Marshal(c, jsontext.WithIndent("  "))
	if err != nil {
		panic(err)
	}
	return b
}

type LoadRootConfigV1Args struct {
	AppInfo  appinfo.AppInfo
	Options  cfgstore.Options
	DirTypes []cfgstore.DirType
}

func LoadRootConfigV1(args LoadRootConfigV1Args) (_ *RootConfigV1, err error) {

	configStores := cfgstore.NewConfigStores(cfgstore.ConfigStoresArgs{
		ConfigStoreArgs: cfgstore.ConfigStoreArgs{
			ConfigSlug:  common.ConfigSlug,
			RelFilepath: common.ConfigFile,
		},
	})

	return cfgstore.LoadRootConfig[RootConfigV1, *RootConfigV1](configStores, cfgstore.RootConfigArgs{
		DirTypes: args.DirTypes,
		Options:  args.Options,
	})

}
