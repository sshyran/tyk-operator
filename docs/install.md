# Tyk Operator Installation

1. Checkout this repository to a branch or tag. For this demo, we will work from the master branch.

```bash
# ssh
git checkout git@github.com:TykTechnologies/tyk-operator.git

# https
git checkout https://github.com/TykTechnologies/tyk-operator.git

# cli
gh repo clone TykTechnologies/tyk-operator
```

2. Before running the operator, the CRDs must be registered with the Kubernetes apiserver

```bash
make install
/Users/ahmet/go/bin/controller-gen "crd:trivialVersions=true,crdVersions=v1" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
/usr/local/bin/kustomize build config/crd | kubectl apply -f -
customresourcedefinition.apiextensions.k8s.io/apidefinitions.tyk.tyk.io created
customresourcedefinition.apiextensions.k8s.io/organizations.tyk.tyk.io created
customresourcedefinition.apiextensions.k8s.io/securitypolicies.tyk.tyk.io created
customresourcedefinition.apiextensions.k8s.io/webhooks.tyk.tyk.io created
```

3. Deploy the operator

3a. OPTIONAL: Override the namespace the operator is to be deployed in
```bash
cd config/default/ && kustomize edit set namespace "changeme" && cd ../..
```

3b. Deploy the operator. This will also install the RBAC manifests from `config/rbac`.
```
make deploy IMG=tykio/tyk-operator:latest
```



## Cleanup

Delete the operator from the namespace

```
kubectl delete ns tyk-operator-system
```

Delete all Tyk Custom Resources

```
kubectl get crds --no-headers=true| awk '/tyk.tyk.io/{print $1}' | xargs kubectl delete crd
```
