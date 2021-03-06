package dubbo

import (
	"context"
	"github.com/apache/dubbo-go/protocol"
)
import (
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
)

type consumerFilter struct{}

func (d *consumerFilter) Invoke(ctx context.Context, invoker protocol.Invoker, invocation protocol.Invocation) protocol.Result {
	methodResourceName := getResourceName(invoker, invocation, getConsumerPrefix())
	interfaceResourceName := ""
	if getInterfaceGroupAndVersionEnabled() {
		interfaceResourceName = getColonSeparatedKey(invoker.GetUrl())
	} else {
		interfaceResourceName = invoker.GetUrl().Service()
	}
	var (
		interfaceEntry *base.SentinelEntry
		methodEntry    *base.SentinelEntry
		b              *base.BlockError
	)

	if !isAsync(invocation) {
		interfaceEntry, b = sentinel.Entry(interfaceResourceName, sentinel.WithResourceType(base.ResTypeRPC), sentinel.WithTrafficType(base.Outbound))
		if b != nil { // blocked
			return consumerDubboFallback(ctx, invoker, invocation, b)
		}
		methodEntry, b = sentinel.Entry(methodResourceName, sentinel.WithResourceType(base.ResTypeRPC), sentinel.WithTrafficType(base.Outbound), sentinel.WithArgs(invocation.Attachments()))
		if b != nil { // blocked
			return consumerDubboFallback(ctx, invoker, invocation, b)
		}
	} else {
		// TODO : Need to implement asynchronous current limiting
		//  unlimited flow for the time being
	}
	ctx = context.WithValue(ctx, InterfaceEntryKey, interfaceEntry)
	ctx = context.WithValue(ctx, MethodEntryKey, methodEntry)
	return invoker.Invoke(ctx, invocation)
}

func (d *consumerFilter) OnResponse(ctx context.Context, result protocol.Result, _ protocol.Invoker, _ protocol.Invocation) protocol.Result {
	if methodEntry := ctx.Value(MethodEntryKey); methodEntry != nil {
		// TODO traceEntry()
		methodEntry.(*base.SentinelEntry).Exit()
	}
	if interfaceEntry := ctx.Value(InterfaceEntryKey); interfaceEntry != nil {
		// TODO traceEntry()
		interfaceEntry.(*base.SentinelEntry).Exit()
	}
	return result
}
