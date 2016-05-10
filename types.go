package main

import "fmt"

type RedisArgs map[string]string

type RedisOption struct {
	Directive string
	Value     string
}

func (o RedisOption) Listify() (list []string) {
	list = append(list, fmt.Sprintf("--%s", o.Directive))
	list = append(list, o.Value)
	return
}

func (a RedisArgs) GetOption(option string) (string, error) {
	val, exists := a[option]
	if !exists {
		return "", fmt.Errorf("Option '%s' not found")
	}
	return val, nil
}

func (a RedisArgs) SetOption(option string, value string) error {
	a[option] = value
	return nil
}

func (a RedisArgs) Listify() (list []string) {
	for k, v := range a {
		list = append(list, fmt.Sprintf("--%s", k))
		list = append(list, v)
	}
	return
}
