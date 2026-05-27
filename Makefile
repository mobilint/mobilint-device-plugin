# Helm binary. Contributors with only Docker can run:
#   make manifests HELM="docker run --rm -v $(PWD):/apps -w /apps alpine/helm"
HELM    ?= helm
CHART   := chart
RELEASE := mobilint-device-plugin
NS      := kube-system
HEADER  := \# Generated from $(CHART) by 'make manifests'. DO NOT EDIT.

# Regenerate the static manifests in deploy/ from the Helm chart.
# The chart is the single source of truth; deploy/ is the no-Helm (kubectl apply) path.
.PHONY: manifests
manifests:
	@echo '$(HEADER)' > deploy/daemonset.yaml
	$(HELM) template $(RELEASE) $(CHART) -n $(NS) -s templates/daemonset.yaml >> deploy/daemonset.yaml
	@echo '$(HEADER)' > deploy/networkpolicy.yaml
	$(HELM) template $(RELEASE) $(CHART) -n $(NS) --set networkPolicy.enabled=true -s templates/networkpolicy.yaml >> deploy/networkpolicy.yaml
	@echo '$(HEADER)' > deploy/nodefeaturerule.yaml
	$(HELM) template $(RELEASE) $(CHART) -n $(NS) --set nodeFeatureDiscovery.enabled=true -s templates/nodefeaturerule.yaml >> deploy/nodefeaturerule.yaml
	@echo "Regenerated deploy/ from $(CHART)"
