# ConsulRegister

Provides a bridge between the docker daemon and consul.

## Registration

New services detected from the docker daemon will be registered in Consul. If a service name can be detected. The following properties are passed along to consul:

- `Name`: Labels with the keys `"service.name", "com.amazonaws.ecs.task-definition-family"` are searched for. If the name does not exist this container is skipped
- `Port`: Looks for the label `"service.port"` which should contain a mapping like `80/tcp` this is used to find the host port currently mapped to the container port for this docker container.
- `Tags`: Looks for the label `"service.tags"`.
- `HealthCheck`: The label `"service.health-check"` is used. The health check is expected in the format: `[Type (HTTP, TCP, Script, TTL)] [Arg] [Interval] [Timeout]`.

## Deregistration

- Services will be deregistered from consul if the docker container exits a running state.
- Containers will be stopped if health checks are marked as critical in consul. Services without health checks will be ignored.
