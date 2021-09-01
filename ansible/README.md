# Push OCP user workload metrics to ACM's Observatorium

**Warning** Strawman implementation with no support!

This ansible playbook deploys pods that push user workload metrics to the Observatorium component of the Red Hat Advanced Cluster Management.

This functionality will be part of ACM in an upcoming release.

## Prerequisites

- Red Hat Advanced Cluster Management (ACM) for Kubernetes 2.2+ (earlier has not been tested)
- Red Hat OpenShift cluster 4.6+ (earlier has not been tested)
    - cluster must be imported to the ACM and healthy
    - make sure the cluster is visible in ACM's grafana
- configured [user workload metrics](https://docs.openshift.com/container-platform/4.6/monitoring/enabling-monitoring-for-user-defined-projects.html#enabling-monitoring-for-user-defined-projects_enabling-monitoring-for-user-defined-projects)
- some user workload `/metrics` available with `ServiceMonitor` or `PodMonitor`
    - make sure the custom metrics are queriable in cluster's Web UI

## Limitations

The playbook duplicates existing deployments of prometheus and ACM's metrics collector.
As a consequence, this playbook shall be re-executed whenever OCP or ACM is upgraded to ensure the secrets and images are up to date.

## Usage

1. install the needed depedencies on your management host

        # sudo dnf install -y jq python-openshift ansible

2. clone the repo locally and install kubernetes galaxy module

        # git clone https://github.com/miminar/sdimetrics.git
        # cd sdimetrics
        # ansible-galaxy collection install -p ansible/library community.kubernetes                                                                                                           

3. ensure you are logged in to the OCP cluster as a cluster-admin

        # oc whoami
        system:admin

5. define custom metrics that shall be pushed to the observatorium

        # cat >custom-metrics.yaml <<EOF
        ---
        match_user_metrics:
         - my_custom_metric1
         - my_custom_metric2
        EOF

6. run the playbook

        # ansible-playbook -e @custom-metrics.yaml ./ansible/playbooks/collect.yaml

7. verify the newly deployed metrics collector pushes metrics to the observatorium, e.g.:

        # oc logs --tail 5 -n open-cluster-management-addon-observability deploy/user-metrics-collector-deployment 
        level=info caller=logger.go:45 ts=2021-09-01T11:55:31.838964921Z component=forwarder component=metricsclient msg="Metrics pushed successfully"
        level=debug caller=logger.go:40 ts=2021-09-01T11:56:01.867855697Z component=forwarder component=metricsclient timeseriesnumber=48
        level=info caller=logger.go:45 ts=2021-09-01T11:56:01.895555047Z component=forwarder component=metricsclient msg="Metrics pushed successfully"
        level=debug caller=logger.go:40 ts=2021-09-01T11:56:31.933359732Z component=forwarder component=metricsclient timeseriesnumber=48
        level=info caller=logger.go:45 ts=2021-09-01T11:56:31.960547474Z component=forwarder component=metricsclient msg="Metrics pushed successfully"
