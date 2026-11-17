package interrupt

import "context"

type contextKeyIsExternalConnection struct{}

func ContextWithIsExternalConnection(ctx context.Context, external ...bool) context.Context {
	isExternal := true
	if len(external) > 0 {
		isExternal = external[0]
	}
	return context.WithValue(ctx, contextKeyIsExternalConnection{}, isExternal)
}

func IsExternalConnectionFromContext(ctx context.Context) bool {
	isExternal, _ := ctx.Value(contextKeyIsExternalConnection{}).(bool)
	return isExternal
}

type contextKeyIsProviderConnection struct{}

func ContextWithIsProviderConnection(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKeyIsProviderConnection{}, true)
}

func IsProviderConnectionFromContext(ctx context.Context) bool {
	return ctx.Value(contextKeyIsProviderConnection{}) != nil
}
