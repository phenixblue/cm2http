# cm2http

cm2http serves one or more data keys from a Kubernetes configMap via HTTP in JSON format

This is useful for low-effort api's of semi-static data.

This originally came about to serve the Kubernetes API CA Certificate since it exists by default on all conformant Kubernetes clusters.

```
$ ./cm2http -h 

A utility to discover and serve the data from a Kubernetes configMap via HTTP

Usage:
  cm2http [flags]

Flags:
      --config string                config file (default is $HOME/.cm2http.yaml)
      --configmap-key string         name of a specific key in the configmap
      --configmap-name string        name of a configmap (default "kube-root-ca.crt")
      --configmap-namespace string   name of the namespace where the configmap is located
      --context string               name of the kubeconfig context to use. Leave blank for default
  -h, --help                         help for cm2http
      --kubeconfig string            name of the kubeconfig file to use. Leave blank for default/in-cluster
      --log-level string             logging level. One of "info" or "debug" (default "info")
```
