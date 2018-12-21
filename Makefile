PROJECT=github.com/lawrencejones/rbac-directory

.PHONY: codegen

codegen:
	vendor/k8s.io/code-generator/generate-groups.sh all \
		$(PROJECT)/pkg/client \
		$(PROJECT)/pkg/apis \
		rbac.gocardless.io:v1alpha1
