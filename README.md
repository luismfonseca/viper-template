# Viper Template

Generates config template file.

## Example usage

Consider this `config.go` file:

```go
type OtherConfig struct {
  X string `mapstructure:"x-value"`
}

//go:generate viper-template --type Config --json config.go
type Config struct {
  Host  string       `mapstructure:"host"`
  Other *OtherConfig `mapstructure:"other"`
}
```

So if you run `go generate`, it will create a file in the same folder named `config.json.template`:
```json
{
  "host": null,
  "other": {
    "x-value": null
  }
}
```

## Installing

Just run:

```
go get github.com/luismfonseca/viper-template
```
