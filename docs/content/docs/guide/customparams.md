---
title: Custom Parameters
weight: 50
---
## Custom Parameters

Using the `{{ param }}` syntax, Pipelines-as-Code let you expand a variable or
the payload body inside a template within your PipelineRun.

By default, there are several variables exposed according to the event. To view
all the variables exposed by default, refer to the documentation on [Authoring
PipelineRuns](../authoringprs#default-parameters).

With the custom parameter, you can specify some custom values to be
replaced inside the template.

{{< hint warning >}}
Utilizing the Tekton PipelineRun parameters feature may generally be the
preferable approach, and custom params expansion should only be used in specific
scenarios where Tekton params cannot be used.
{{< /hint >}}

As an example here is a custom variable in the Repository CR `spec`:

```yaml
spec:
  params:
    - name: company
      value: "My Beautiful Company"
```

The variable name `{{ company }}` will be replaced by `My Beautiful Company`
anywhere inside your `PipelineRun` (including the remotely fetched task).

Alternatively, the value can be retrieved from a Kubernetes Secret.
For instance, the following code will retrieve the value for the company
`parameter` from a secret named `my-secret` and the key `companyname`:

```yaml
spec:
  params:
    - name: company
      secret_ref:
        name: my-secret
        key: companyname
```

{{< hint info >}}

- If you have a `value` and a `secret_ref` defined, the `value` will be used.
- If you don't have a `value` or a `secret_ref` the parameter will not be
  parsed, it will be shown as `{{ param }}` in the `PipelineRun`.
- If you don't have a `name` in the `params` the parameter will not parsed.
- If you have multiple `params` with the same `name` the last one will be used.
{{< /hint >}}

### CEL filtering on custom parameters

You can define a `param` to only apply the custom parameters expansion when some
conditions has been matched on a `filter`:

```yaml
spec:
  params:
    - name: company
      value: "My Beautiful Company"
      filter: pac.event_type == "pull_request"
```

The `pac` prefix contains all the values as set by default in the templates
variables. Refer to the [Authoring PipelineRuns](../authoringprs) documentation
for all the variable exposed by default.

The body of the payload is exposed inside the `body` prefix.

For example if you are running a Pull Request on GitHub pac will receive a
payload which has this kind of json:

```json
{
  "action": "opened",
  "number": 79,
  // .... more data
}
```

The filter can then do something like this:

```yaml
spec:
  params:
    - name: company
      value: "My Beautiful Company"
      filter: body.action == "opened" && pac.event_type == "pull_request"
```

The payload of the event contains much more information that can be used with
the CEL filter. To see the specific payload content for your provider, refer to
the API documentation

You can have multiple `params` with the same name and different filters, the
first param that matches the filter will be picked up. This let you have
different output according to different event, and for example combine a push
and a pull request event.

{{< hint info >}}

- [GitHub Documentation for webhook events](https://docs.github.com/webhooks-and-events/webhooks/webhook-events-and-payloads?actionType=auto_merge_disabled#pull_request)
- [GitLab Documentation for webhook events](https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html)
{{< /hint >}}
