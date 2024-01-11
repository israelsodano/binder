package binder

import (
	"encoding/json"
	"fmt"
	"log"

	"rogchap.com/v8go"
)

func ExecuteScriptTyped[T interface{}](s []byte, d []byte) (T, error) {
	var t T
	r := ExecuteScript(s, d)
	err := json.Unmarshal(r, &t)
	return t, err
}

func ExecuteScript(s []byte, d []byte) (rs []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
			rs = []byte(`""`)
		}
	}()
	iso := v8go.NewIsolate()
	defer iso.Dispose()
	ctx := v8go.NewContext(iso)
	defer ctx.Close()
	ds, err := v8go.JSONParse(ctx, string(d))
	if err != nil {
		log.Println(err.Error())
		return []byte(`""`)
	}
	ctx.Global().Set("ctx", ds)
	val, err := ctx.RunScript(string(s), "resolve.js")
	if err != nil {
		log.Println(err.Error())
		return []byte(`""`)
	}
	if val.IsObject() {
		ss, err := v8go.JSONStringify(ctx, val)
		if err != nil {
			log.Println(err.Error())
			return []byte(`""`)
		}
		return []byte(ss)
	}
	if val.IsString() {
		return []byte(fmt.Sprintf(`"%s"`, val.String()))
	}

	return []byte(val.String())
}
