Mixed Providers
===============

Mixed providers combine multiple provider capabilities in one executable.

- [`provider-mixed-aws-ec2`](./provider-mixed-aws-ec2/README.md)
- [`provider-mixed-azure-compute`](./provider-mixed-azure-compute/README.md)
- [`provider-mixed-gcp-compute`](./provider-mixed-gcp-compute/README.md)

## Role

Mixed providers are useful when one cloud integration needs both:

- `inventory`
  - discover hosts and emit stable metadata
- `connector`
  - prepare cloud-specific transport or execution plans from that metadata

This keeps the upstream identity and connector planning in the same provider implementation.

## Current Providers

### `provider-mixed-aws-ec2`

- combines EC2 inventory with AWS SSM / EICE connector behavior

### `provider-mixed-gcp-compute`

- combines Compute Engine inventory with the `gcp-iap` connector

### `provider-mixed-azure-compute`

- combines Azure VM inventory with the `azure-bastion` connector

