package proto5server

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/internal/fwserver"
	"github.com/hashicorp/terraform-plugin-framework/internal/privatestate"
	"github.com/hashicorp/terraform-plugin-framework/internal/testing/testprovider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestServerReadResource(t *testing.T) {
	t.Parallel()

	testType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"test_computed": tftypes.String,
			"test_required": tftypes.String,
		},
	}

	testCurrentStateValue := testNewDynamicValue(t, testType, map[string]tftypes.Value{
		"test_computed": tftypes.NewValue(tftypes.String, nil),
		"test_required": tftypes.NewValue(tftypes.String, "test-currentstate-value"),
	})

	testEmptyDynamicValue := testNewDynamicValue(t, tftypes.Object{}, nil)

	testNewStateDynamicValue := testNewDynamicValue(t, testType, map[string]tftypes.Value{
		"test_computed": tftypes.NewValue(tftypes.String, "test-newstate-value"),
		"test_required": tftypes.NewValue(tftypes.String, "test-currentstate-value"),
	})

	testNewStateRemovedDynamicValue, _ := tfprotov5.NewDynamicValue(testType, tftypes.NewValue(testType, nil))

	testSchema := tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"test_computed": {
				Computed: true,
				Type:     types.StringType,
			},
			"test_required": {
				Required: true,
				Type:     types.StringType,
			},
		},
	}

	testCases := map[string]struct {
		server           *Server
		request          *tfprotov5.ReadResourceRequest
		expectedError    error
		expectedResponse *tfprotov5.ReadResourceResponse
	}{
		"no-schema": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						ResourcesMethod: func(_ context.Context) []func() resource.Resource {
							return []func() resource.Resource{
								func() resource.Resource {
									return &testprovider.Resource{
										GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
											return tfsdk.Schema{}, nil
										},
										MetadataMethod: func(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
											resp.TypeName = "test_resource"
										},
									}
								},
							}
						},
					},
				},
			},
			request: &tfprotov5.ReadResourceRequest{
				CurrentState: testEmptyDynamicValue,
				TypeName:     "test_resource",
			},
			expectedResponse: &tfprotov5.ReadResourceResponse{
				NewState: testEmptyDynamicValue,
			},
		},
		"request-currentstate": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						ResourcesMethod: func(_ context.Context) []func() resource.Resource {
							return []func() resource.Resource{
								func() resource.Resource {
									return &testprovider.Resource{
										GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
											return testSchema, nil
										},
										MetadataMethod: func(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
											resp.TypeName = "test_resource"
										},
										ReadMethod: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
											var data struct {
												TestComputed types.String `tfsdk:"test_computed"`
												TestRequired types.String `tfsdk:"test_required"`
											}

											resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

											if data.TestRequired.Value != "test-currentstate-value" {
												resp.Diagnostics.AddError("unexpected req.State value: %s", data.TestRequired.Value)
											}
										},
									}
								},
							}
						},
					},
				},
			},
			request: &tfprotov5.ReadResourceRequest{
				CurrentState: testCurrentStateValue,
				TypeName:     "test_resource",
			},
			expectedResponse: &tfprotov5.ReadResourceResponse{
				NewState: testCurrentStateValue,
			},
		},
		"request-providermeta": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.ProviderWithMetaSchema{
						Provider: &testprovider.Provider{
							ResourcesMethod: func(_ context.Context) []func() resource.Resource {
								return []func() resource.Resource{
									func() resource.Resource {
										return &testprovider.Resource{
											GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
												return tfsdk.Schema{}, nil
											},
											MetadataMethod: func(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
												resp.TypeName = "test_resource"
											},
											ReadMethod: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
												var data struct {
													TestComputed types.String `tfsdk:"test_computed"`
													TestRequired types.String `tfsdk:"test_required"`
												}

												resp.Diagnostics.Append(req.ProviderMeta.Get(ctx, &data)...)

												if data.TestRequired.Value != "test-currentstate-value" {
													resp.Diagnostics.AddError("unexpected req.ProviderMeta value: %s", data.TestRequired.Value)
												}
											},
										}
									},
								}
							},
						},
						GetMetaSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
							return testSchema, nil
						},
					},
				},
			},
			request: &tfprotov5.ReadResourceRequest{
				CurrentState: testEmptyDynamicValue,
				ProviderMeta: testCurrentStateValue,
				TypeName:     "test_resource",
			},
			expectedResponse: &tfprotov5.ReadResourceResponse{
				NewState: testEmptyDynamicValue,
			},
		},
		"request-private": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						ResourcesMethod: func(_ context.Context) []func() resource.Resource {
							return []func() resource.Resource{
								func() resource.Resource {
									return &testprovider.Resource{
										GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
											return tfsdk.Schema{}, nil
										},
										MetadataMethod: func(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
											resp.TypeName = "test_resource"
										},
										ReadMethod: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
											expected := `{"pKeyOne": {"k0": "zero", "k1": 1}}`
											got, diags := req.Private.GetKey(ctx, "providerKey")

											resp.Diagnostics.Append(diags...)

											if string(got) != expected {
												resp.Diagnostics.AddError(
													"Unexpected req.Private Value",
													fmt.Sprintf("expected %q, got %q", expected, got),
												)
											}
										},
									}
								},
							}
						},
					},
				},
			},
			request: &tfprotov5.ReadResourceRequest{
				CurrentState: testEmptyDynamicValue,
				TypeName:     "test_resource",
				Private: privatestate.MustMarshalToJson(map[string][]byte{
					".frameworkKey": []byte(`{"fKeyOne": {"k0": "zero", "k1": 1}}`),
					"providerKey":   []byte(`{"pKeyOne": {"k0": "zero", "k1": 1}}`),
				}),
			},
			expectedResponse: &tfprotov5.ReadResourceResponse{
				NewState: testEmptyDynamicValue,
				Private: privatestate.MustMarshalToJson(map[string][]byte{
					".frameworkKey": []byte(`{"fKeyOne": {"k0": "zero", "k1": 1}}`),
					"providerKey":   []byte(`{"pKeyOne": {"k0": "zero", "k1": 1}}`),
				}),
			},
		},
		"response-diagnostics": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						ResourcesMethod: func(_ context.Context) []func() resource.Resource {
							return []func() resource.Resource{
								func() resource.Resource {
									return &testprovider.Resource{
										GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
											return testSchema, nil
										},
										MetadataMethod: func(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
											resp.TypeName = "test_resource"
										},
										ReadMethod: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
											resp.Diagnostics.AddWarning("warning summary", "warning detail")
											resp.Diagnostics.AddError("error summary", "error detail")
										},
									}
								},
							}
						},
					},
				},
			},
			request: &tfprotov5.ReadResourceRequest{
				CurrentState: testCurrentStateValue,
				TypeName:     "test_resource",
			},
			expectedResponse: &tfprotov5.ReadResourceResponse{
				Diagnostics: []*tfprotov5.Diagnostic{
					{
						Severity: tfprotov5.DiagnosticSeverityWarning,
						Summary:  "warning summary",
						Detail:   "warning detail",
					},
					{
						Severity: tfprotov5.DiagnosticSeverityError,
						Summary:  "error summary",
						Detail:   "error detail",
					},
				},
				NewState: testCurrentStateValue,
			},
		},
		"response-state": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						ResourcesMethod: func(_ context.Context) []func() resource.Resource {
							return []func() resource.Resource{
								func() resource.Resource {
									return &testprovider.Resource{
										GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
											return testSchema, nil
										},
										MetadataMethod: func(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
											resp.TypeName = "test_resource"
										},
										ReadMethod: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
											var data struct {
												TestComputed types.String `tfsdk:"test_computed"`
												TestRequired types.String `tfsdk:"test_required"`
											}

											resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

											data.TestComputed = types.String{Value: "test-newstate-value"}

											resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
										},
									}
								},
							}
						},
					},
				},
			},
			request: &tfprotov5.ReadResourceRequest{
				CurrentState: testCurrentStateValue,
				TypeName:     "test_resource",
			},
			expectedResponse: &tfprotov5.ReadResourceResponse{
				NewState: testNewStateDynamicValue,
			},
		},
		"response-state-removeresource": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						ResourcesMethod: func(_ context.Context) []func() resource.Resource {
							return []func() resource.Resource{
								func() resource.Resource {
									return &testprovider.Resource{
										GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
											return testSchema, nil
										},
										MetadataMethod: func(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
											resp.TypeName = "test_resource"
										},
										ReadMethod: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
											resp.State.RemoveResource(ctx)
										},
									}
								},
							}
						},
					},
				},
			},
			request: &tfprotov5.ReadResourceRequest{
				CurrentState: testCurrentStateValue,
				TypeName:     "test_resource",
			},
			expectedResponse: &tfprotov5.ReadResourceResponse{
				NewState: &testNewStateRemovedDynamicValue,
			},
		},
		"response-private": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						ResourcesMethod: func(_ context.Context) []func() resource.Resource {
							return []func() resource.Resource{
								func() resource.Resource {
									return &testprovider.Resource{
										GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
											return tfsdk.Schema{}, nil
										},
										MetadataMethod: func(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
											resp.TypeName = "test_resource"
										},
										ReadMethod: func(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
											diags := resp.Private.SetKey(ctx, "providerKey", []byte(`{"key": "value"}`))

											resp.Diagnostics.Append(diags...)
										},
									}
								},
							}
						},
					},
				},
			},
			request: &tfprotov5.ReadResourceRequest{
				CurrentState: testEmptyDynamicValue,
				TypeName:     "test_resource",
			},
			expectedResponse: &tfprotov5.ReadResourceResponse{
				NewState: testEmptyDynamicValue,
				Private: privatestate.MustMarshalToJson(map[string][]byte{
					"providerKey": []byte(`{"key": "value"}`),
				}),
			},
		},
	}

	for name, testCase := range testCases {
		name, testCase := name, testCase

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := testCase.server.ReadResource(context.Background(), testCase.request)

			if diff := cmp.Diff(testCase.expectedError, err); diff != "" {
				t.Errorf("unexpected error difference: %s", diff)
			}

			if diff := cmp.Diff(testCase.expectedResponse, got); diff != "" {
				t.Errorf("unexpected response difference: %s", diff)
			}
		})
	}
}
