# kngrok

*ken-grok*

## What is kngrok?

kngrok is a Kubernetes controller to operate [ngrok][ngrok] Tunnel in Kubernetes clusters. This controller leverages the Service with the specified [LoadBalancerClass][LoadBalancerClass] and will act when the Service with LoadBalancerClass matches with the service LoadBalancer class name the controller watch to.

## Getting Started



## License

This project is licensed under Apache License 2.0, see [LICENSE](./LICENSE).

<!-- Reference -->
[ngrok]: https://ngrok.io/
[LoadBalancerClass]: https://kubernetes.io/docs/concepts/services-networking/service/#load-balancer-class
