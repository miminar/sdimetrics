# Periodic systemd reloads

The daemonset reloads systemd daemon roughly each 24 hours.

## Motivation

There are reported issues of node instability on OCP 4 in some environments and with particular workloads. The symptoms are the following:

- after some time of proper function (1-3 weeks), zombie processes suddenly start to accumulate on the node
- since then, all the new pods scheduled on the node are stuck in `ContainerCreating` phase
- when the stuck pod is described, the displayed error contains `Argument list too long`

Please find the most up to date information at [bz#1994444](https://bugzilla.redhat.com/show_bug.cgi?id=1994444).

Usually, if acted upon early enough, the situation can be mitigated with the following sequence of commands executed on the affected node:

    # sudo su
    # systemctl daemon-reload
    # systemctl reset-failed
    # # does not affect running pods, it cleans up the zombies
    # systemctl restart crio.service

However, if it is too late, the commands fail with a similar message:

    Failed to reload daemon: Connection reset by peer

In that case, the affected node must be rebooted.

The daemonset is an alternative pre-emptive and short-term solution to the problem. It performs the above commands periodically. They have been observed to contribute to node's stability.

The daemonset shall be deployed only if the issue above is occuring on the cluster.

## Usage

As a `cluster-admin` run the following from your management host:

    # nm=systemd-reloader
    # oc new-project "$nm"
    # # optionally, restrict the daemonset to the nodes matching the selector; e.g. run only on OCS/ODF nodes
    # oc annotate namespace/"$nm" openshift.io/node-selector="cluster.ocs.openshift.io/openshift-storage="
    # oc apply -f https://raw.githubusercontent.com/miminar/sdimetrics/master/systemd-reloader/sa-rolebindings.yaml
    # oc adm policy add-cluster-role-to-user system:node-reader -z systemd-reloader
    # oc apply -f https://raw.githubusercontent.com/miminar/sdimetrics/master/systemd-reloader/ds-systemd-reloader.yaml

## Uninstallation

    # oc delete project systemd-reloader
