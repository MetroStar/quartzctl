// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

// EksToken represents an EKS authentication token and its JSON representation.
type EksToken struct {
	Token      token.Token // The EKS authentication token.
	JsonString string      // The JSON representation of the token.
}

// EksKubeconfigInfo retrieves the kubeconfig information for an EKS cluster.
// It returns the kubeconfig details, an EKS token, and an error if any occurs.
func (c *AwsClient) EksKubeconfigInfo(ctx context.Context) (KubeconfigInfo, EksToken, error) {
	cluster, err := c.DescribeEksCluster(ctx)
	if err != nil {
		return KubeconfigInfo{}, EksToken{}, err
	}

	t, err := c.generateEksUserToken()
	if err != nil {
		return KubeconfigInfo{}, t, err
	}

	return KubeconfigInfo{
		Context:              *cluster.Cluster.Arn,
		User:                 *cluster.Cluster.Arn,
		Cluster:              *cluster.Cluster.Arn,
		Endpoint:             *cluster.Cluster.Endpoint,
		CertificateAuthority: *cluster.Cluster.CertificateAuthority.Data,
		Token:                t.Token.Token,
		Expiration:           t.Token.Expiration,
	}, t, nil
}

// DescribeEksCluster describes the EKS cluster associated with the client.
// It returns the cluster details or an error if the operation fails.
func (c *AwsClient) DescribeEksCluster(ctx context.Context) (*eks.DescribeClusterOutput, error) {
	return c.sdk.Eks().DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: &c.id,
	})
}

// generateEksUserToken generates an authentication token for the EKS cluster.
// It returns the token, its JSON representation, and an error if the operation fails.
func (c *AwsClient) generateEksUserToken() (EksToken, error) {
	g, err := c.sdk.EksTokenGenerator()
	if err != nil {
		return EksToken{}, err
	}

	t, err := g.GetWithOptions(&token.GetTokenOptions{
		ClusterID: c.id,
		Region:    c.region,
	})

	if err != nil {
		return EksToken{}, err
	}

	return EksToken{
		Token:      t,
		JsonString: g.FormatJSON(t),
	}, nil
}
