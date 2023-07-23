# benchmark-operator

Developing benchmarks to assess different kinds of Kubernetes performance.
We likely will choose different metrics that are important for HPC.
Note that I haven't started the operator yet because I'm [testing ideas for the design](hack/test).

## Design Thinking

### Database for Metric Storage

I want to try creating a consistent database that can be used to store metrics across runs. In the space of an operator,
this means we can't clean it up when the specific metric is deleted, but rather it should be owned by the namespace.
I'm not sure how to do that but will think about ideas. Worst case, we have the user deploy the database in the same namespace
separately. Best case, we can manage it for them, or (better) not require it at all.
I don't want anything complicated (I don't want to re-create prometheus or a monitoring service!)

### Kubernetes Objects

JobSet gives us a lot of flexibility to deploy different kinds of applications or services alongside one another. I'm wondering if we can
have some design where a replicated job corresponds to one metric, and then one run can include one or more metrics.

### Metrics

The following metrics are interesting, and here is how we might measure them with an operator:

 - **storage**: the operator could be provided information about a PVC to create, and then create it and run different IO tools on it. Each different IO app would correspond to a different plugin.
 - **time**: In that an appliction (container) can be run, meaning the application is the main entrypoint that goes from start to finish, we could likely time processes that we know correspond to our application.
 - **performance**: More generally, I wonder if we can add the SYS_PTRACE capability to containers in the same pod and then be able to monitor processes from one container into another? If we are able to know one or more processes of interest, and find tools that can give meaningful metrics from the processes, that could be a cool setup.
 - **others**: There are likely others (and I need to think about it)

I'm going to start with the generic interface for a metric, which will provide common interfaces for the operator to make calls,
and then metric "flavors" will likely correspond to the above. I don't know if any of this will work, but I don't care, because
it's fun to try and learn. ;)

## Development

### Creation

```bash
mkdir htcondor-operator
cd htcondor-operator/
operator-sdk init --domain flux-framework.org --repo github.com/converged-computing/benchmark-operator
operator-sdk create api --version v1alpha1 --kind Metric --resource --controller
```

## License

HPCIC DevTools is distributed under the terms of the MIT license.
All new contributions must be made under this license.

See [LICENSE](https://github.com/converged-computing/cloud-select/blob/main/LICENSE),
[COPYRIGHT](https://github.com/converged-computing/cloud-select/blob/main/COPYRIGHT), and
[NOTICE](https://github.com/converged-computing/cloud-select/blob/main/NOTICE) for details.

SPDX-License-Identifier: (MIT)

LLNL-CODE- 842614
