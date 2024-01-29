package binder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/buger/jsonparser"
	"github.com/ledongthuc/goterators"
	"golang.org/x/exp/slices"
)

func getobjvariables(s []byte) [][]byte {
	rg, _ := regexp.Compile(`"O{(.*?)}"`)
	r := rg.FindAll(s, -1)
	return r
}

func getstrvariables(s []byte) [][]byte {
	rg, _ := regexp.Compile(`\${(.*?)}`)
	r := rg.FindAll(s, -1)
	return r
}

func getmapvariables(s []byte) [][]byte {
	rg, _ := regexp.Compile(`M{(.*?)}`)
	r := rg.FindAll(s, -1)
	return r
}

func getkeys(k string) []string {
	r := strings.NewReplacer("O{", "", "${", "", "}", "", "[", ".[", "]", "].", "\"", "")
	k = r.Replace(k)
	kys := strings.Split(k, ".")
	return goterators.Filter(kys, func(i string) bool {
		return i != ""
	})
}

func distinc(s [][]byte) []string {
	ss := []string{}
	for _, v := range s {
		ss = append(ss, string(v))
	}
	n := []string{}
	for i, v := range s {
		if i == slices.Index(ss, string(v)) {
			n = append(n, string(v))
		}
	}
	return n
}

var Cmap = map[string]string{}

func Bind(s []byte, ctx []byte) []byte {
	return bindmap(
		bindobj(
			bindstr(
				visitarrays(s, ctx), ctx), ctx), Cmap)
}

func JSONMarshal(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

func bindobj(s []byte, ctx []byte) []byte {
	vars := distinc(getobjvariables(s))
	rs := []string{}
	for _, v := range vars {
		rs = append(rs, v)
		ks := getkeys(v)
		rv, _, _, err := jsonparser.Get(ctx, ks...)
		if err != nil {
			fmt.Printf("key: '%v', err: '%v'\n", v, err.Error())
			rs = append(rs, "null")
			continue
		}
		rs = append(rs, string(rv))
	}
	rps := strings.NewReplacer(rs...)
	ss := rps.Replace(string(s))
	return []byte(ss)
}

func bindstr(s []byte, ctx []byte) []byte {
	vars := distinc(getstrvariables(s))
	rs := []string{}
	for _, v := range vars {
		rs = append(rs, v)
		ks := getkeys(v)
		rv, _, _, err := jsonparser.Get(ctx, ks...)
		if err != nil {
			fmt.Printf("key: '%v', err: '%v'\n", v, err.Error())
		}
		rs = append(rs, string(rv))
	}
	rps := strings.NewReplacer(rs...)
	ss := rps.Replace(string(s))
	return []byte(ss)
}

func bindmap(s []byte, m map[string]string) []byte {
	mvars := distinc(getmapvariables(s))
	if len(mvars) == 0 {
		return []byte(s)
	}
	rs := []string{}
	r := strings.NewReplacer("M{", "", "}", "")
	for _, v := range mvars {
		rs = append(rs, v)
		mv := m[r.Replace(v)]
		rs = append(rs, mv)
	}
	rps := strings.NewReplacer(rs...)
	ss := rps.Replace(string(s))
	return []byte(ss)
}

func visitarrays(s []byte, ctx []byte) []byte {
	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	jsonparser.ObjectEach(s, func(key, value []byte, dataType jsonparser.ValueType, _ int) error {
		wg.Add(1)
		go func() {
			var err error
			defer wg.Done()
			if dataType == jsonparser.Array {
				value = bindarray(value, ctx)
				mu.Lock()
				defer mu.Unlock()
				s, err = jsonparser.Set(s, value, string(key))
				if err != nil {
					panic(err)
				}
				return
			}
			if dataType == jsonparser.Object {
				value = visitarrays(value, ctx)
				mu.Lock()
				defer mu.Unlock()
				s, err = jsonparser.Set(s, value, string(key))
				if err != nil {
					panic(err)
				}
				return
			}
			if dataType == jsonparser.String {
				if js := string(value); strings.HasPrefix(js, "$F") {
					js = strings.ReplaceAll(js, "$F", "")
					value = ExecuteScript([]byte(js), ctx)
					mu.Lock()
					defer mu.Unlock()
					s, err = jsonparser.Set(s, value, string(key))
					if err != nil {
						panic(err)
					}
					return
				}
			}
		}()
		return nil
	})
	wg.Wait()
	return s
}

func BindTemplateArray(temp []byte, ctx []byte) []byte {
	it, err := jsonparser.GetString(temp, "it")
	if err != nil || it == "" {
		return []byte(fmt.Sprintf("[%v]", string(temp)))
	}
	i, err := jsonparser.GetString(temp, "i")
	if err != nil || i == "" {
		return []byte(fmt.Sprintf("[%v]", string(temp)))
	}
	arr, dt, _, _ := jsonparser.Get(ctx, getkeys(it)...)
	if dt != jsonparser.Array {
		bit := Bind([]byte(it), ctx)
		arr, dt, _, _ = jsonparser.Get(bit)
		if dt != jsonparser.Array {
			panic(errors.New("it property reference of array is not valid"))
		}
	}
	idx := 0
	temp = jsonparser.Delete(temp, "it")
	temp = jsonparser.Delete(temp, "i")
	strtmp := string(temp)
	i = "[" + i + "]"
	sarr := []string{}
	jsonparser.ArrayEach(arr, func(_ []byte, _ jsonparser.ValueType, _ int, _ error) {
		stridx := fmt.Sprintf("[%v]", idx)
		bstrtemp := strings.ReplaceAll(strtmp, i, stridx)
		btemp := visitarrays([]byte(bstrtemp), ctx)
		sarr = append(sarr, string(btemp))
		idx++
	})
	return []byte(fmt.Sprintf("[%s]", strings.Join(sarr, ",")))
}

func bindarray(s []byte, ctx []byte) []byte {
	temp, _, _, err := jsonparser.Get(s, "[0]")
	if err != nil {
		return s
	}
	_, err = jsonparser.GetString(temp, "it")
	if err != nil {
		sarr := []string{}
		jsonparser.ArrayEach(s, func(value []byte, _ jsonparser.ValueType, _ int, _ error) {
			value = visitarrays(value, ctx)
			sarr = append(sarr, string(value))
		})
		return []byte(fmt.Sprintf("[%s]", strings.Join(sarr, ",")))
	}
	return BindTemplateArray(temp, ctx)
}
