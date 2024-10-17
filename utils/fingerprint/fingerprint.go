package fingerprint

import (
	"fmt"
	"slices"
	"strings"
)

type DimensionHierarchyNode struct {
	// labels that map to this node in dimension hierarchy
	labels []string

	// List of potential subhierachies in the order of preference.
	// Eg: k8s.cluster.name can have subhierarchies of k8s.namespace.name or k8s.node.name - the 2 ways of grouping/organizing k8s resources.
	// In most cases this list will have only one entry
	subHierachies []DimensionHierarchyNode
}

type IdLabelValue struct {
	Label string
	Value any
}

// Returns list of dimension labels for a set of attributes for a DimensionHierarchy
func (node *DimensionHierarchyNode) Identifier(attributes map[string]any) []IdLabelValue {
	result := []IdLabelValue{}

	for _, l := range node.labels {
		if lVal, exists := attributes[l]; exists {
			result = append(result, IdLabelValue{
				Label: l,
				Value: lVal,
			})
			break
		}
	}

	for _, s := range node.subHierachies {
		subLabels := s.Identifier(attributes)
		if len(subLabels) > 0 {
			result = append(result, subLabels...)
			break
		}
	}

	return result
}

// Get list of synonymous labels in a hierarchy for `attribute` or `nil`
func (node *DimensionHierarchyNode) Synonyms(attribute string) []string {
	if node == nil {
		return nil
	}

	if slices.Contains(node.labels, attribute) {
		return node.labels[:]
	}

	for _, h := range node.subHierachies {
		synonyms := h.Synonyms(attribute)
		if len(synonyms) > 0 {
			return synonyms
		}
	}

	return nil
}

// TODO(Raj/Nitya): Consider parsing this stuff out from json
func ResourceHierarchy() *DimensionHierarchyNode {
	return &DimensionHierarchyNode{
		labels: []string{
			"cloud.provider",
		},
		subHierachies: []DimensionHierarchyNode{{
			labels: []string{"cloud.account.id"},

			subHierachies: []DimensionHierarchyNode{{
				labels: []string{"gcp.project"},

				subHierachies: []DimensionHierarchyNode{{
					labels: []string{
						"cloud.region",
						"aws.region",
					},

					subHierachies: []DimensionHierarchyNode{{
						labels: []string{
							"cloud.platform",
							"source_type",
						},

						subHierachies: []DimensionHierarchyNode{{
							labels: []string{
								"k8s.cluster.name",
								"k8s.cluster.uid",
								"aws.ecs.cluster.arn",
							},

							subHierachies: []DimensionHierarchyNode{
								// Logical/service oriented view
								{
									labels: []string{
										"service.namespace",
										"k8s.namespace.name",
										"ec2.tag.service-group", // is this standard enough?
									},

									subHierachies: []DimensionHierarchyNode{{
										labels: []string{
											"service.name",
											"cloudwatch.log.group.name",
											"k8s.deployment.name",
											"k8s.deployment.uid",
											"k8s.statefulset.name",
											"k8s.statefulset.uid",
											"k8s.daemonset.name",
											"k8s.daemonset.uid",
											"k8s.job.name",
											"k8s.job.uid",
											"k8s.cronjob.name",
											"k8s.cronjob.uid",
											"faas.name",
											"ec2.tag.service", // is this standard enough?
										},

										subHierachies: []DimensionHierarchyNode{{
											labels: []string{
												"deployment.environment",
												"ec2.tag.env-short", // is this standard enough?
												"ec2.tag.env",       // is this standard enough?
												"env",
											},

											subHierachies: []DimensionHierarchyNode{{
												labels: []string{
													"service.instance.id",
													"k8s.pod.name",
													"k8s.pod.uid",
													"aws.ecs.task.id",
													"aws.ecs.task.arn",
													"cloudwatch.log.stream",
													"cloud.resource_id",
													"faas.instance",
													"host.id",
													"host.name",
													"host.ip",
													"host",
												},

												subHierachies: []DimensionHierarchyNode{{
													labels: []string{
														"k8s.container.name",
														"container.name",
														"container_name",
													},

													subHierachies: []DimensionHierarchyNode{{
														labels: []string{
															"component",
														},
													}},
												}},
											}},
										}},
									}},
								},

								// Node oriented view
								{
									labels: []string{"cloud.availability_zone"},

									subHierachies: []DimensionHierarchyNode{{
										labels: []string{
											"k8s.node.name",
											"k8s.node.uid",
											"host.id",
											"host.name",
											"host.ip",
											"host",
										},

										subHierachies: []DimensionHierarchyNode{{
											labels: []string{
												"k8s.pod.name",
												"k8s.pod.uid",
												"aws.ecs.task.id",
												"aws.ecs.task.arn",
											},

											subHierachies: []DimensionHierarchyNode{{
												labels: []string{
													"k8s.container.name",
													"container.name",
												},
											}},
										}},
									}},
								}},
						}},
					}},
				}},
			}},
		}},
	}
}

// Calculates fingerprint for attributes that when sorted would keep fingerprints
// for the same set of attributes next to each other while also colocating
// entries at all levels of the hierarchy
// For example, fingerprints like "k8s.deployment.name=webserver;k8s.pod.name=webserver-0"
// will calculate logs for each webserver pod while also calculate all logs for the webserver deployment
func CalculateFingerprint(
	attributes map[string]any, hierarchy *DimensionHierarchyNode,
) string {
	id := hierarchy.Identifier(attributes)

	fingerprintParts := []string{}
	for _, idLabel := range id {
		fingerprintParts = append(
			fingerprintParts, fmt.Sprintf("%s=%s", idLabel.Label, idLabel.Value),
		)
	}

	hash := FingerprintHash(attributes)
	fingerprintParts = append(fingerprintParts, fmt.Sprintf("%s=%v", "hash", hash))

	return strings.Join(fingerprintParts, ";")
}
