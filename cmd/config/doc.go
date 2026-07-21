// Package config holds the logic behind the SDK's "config" command family.
//
// It knows nothing about cobra. The commands themselves stay in the parent
// cmd package — cmd.ConfigCmd and cmd.ConfigDefaultCmd — and are thin shells:
// they own the flags, call one function here, and render the structured
// result it returns. Everything else lives in this package:
//
//	Show                      the merged view across every config layer
//	Update / Delete           key-level mutations of a target file
//	Append / RemoveFrom       element-level mutations of an array field
//	Apply                     all of the above, applied atomically
//	Default                   write the application's shipped default file
//	RegisterDefault           where the hosting application hands that file in
//
// Nothing here writes to stdout or builds display strings, so a second front
// end — an HTTP handler, a library caller, a TUI — can reuse the same
// functions without dragging the table renderer along. The renderers are
// exported too (RenderShowTable, RenderChangeReport, RenderDefaultReport) for
// callers that do want the CLI's exact output.
//
// The import of the SDK's own configuration package is aliased to sdkconfig
// throughout, because this package is also called config and the two are
// easily confused: sdkconfig loads configuration into viper, this package
// inspects and edits the files that configuration comes from.
package config
