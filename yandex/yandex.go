package yandex

import (
	"context"
	"fmt"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/dns/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/iam/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
)

type YandexCloudDNSProvider struct {
	dnsZoneId   string
	credentials ycsdk.Credentials
}

func NewYandexCloudDNSProvider(dnsZoneId string, serviceAccountId string, id string, encryption string, publicKey string, privateKey string) (*YandexCloudDNSProvider, error) {
	keyAlgorithm := iam.Key_RSA_2048
	if encryption == "RSA_4096" {
		keyAlgorithm = iam.Key_RSA_4096
	}

	credentials, err := ycsdk.ServiceAccountKey(&iamkey.Key{
		Id:           id,
		Subject:      &iamkey.Key_ServiceAccountId{ServiceAccountId: serviceAccountId},
		CreatedAt:    nil,
		Description:  "",
		KeyAlgorithm: keyAlgorithm,
		PublicKey:    publicKey,
		PrivateKey:   privateKey,
	})

	if err != nil {
		return nil, fmt.Errorf("unable to create ServiceAccountKey: %v", err)
	}

	return &YandexCloudDNSProvider{
		dnsZoneId:   dnsZoneId,
		credentials: credentials,
	}, nil
}

// Present creates a TXT record to fulfill DNS-01 challenge.
func (y *YandexCloudDNSProvider) Present(fqdn string, value string) error {
	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: y.credentials,
	})

	if err != nil {
		return fmt.Errorf("unable to create SDK: %v", err)
	}

	recordSetRequest := dns.GetDnsZoneRecordSetRequest{
		DnsZoneId: y.dnsZoneId,
		Name:      fqdn,
		Type:      "TXT",
	}

	recordSet, err := sdk.DNS().DnsZone().GetRecordSet(context.Background(), &recordSetRequest)

	data := []string{value}
	if recordSet != nil {
		data = append(recordSet.Data, value)

		req := dns.UpdateRecordSetsRequest{
			DnsZoneId: y.dnsZoneId,
			Deletions: []*dns.RecordSet{
				recordSet,
			},
		}
		_, err := sdk.DNS().DnsZone().UpdateRecordSets(context.Background(), &req)

		if err != nil {
			return fmt.Errorf("unable to delete DNS record for: %s, with code: %v", fqdn, err)
		}
	}

	req := dns.UpdateRecordSetsRequest{
		DnsZoneId: y.dnsZoneId,
		Additions: []*dns.RecordSet{
			{
				Name: fqdn,
				Type: "TXT",
				Ttl:  60,
				Data: data,
			},
		},
	}

	_, err = sdk.DNS().DnsZone().UpdateRecordSets(context.Background(), &req)

	if err != nil {
		return fmt.Errorf("unable to create DNS record for: %s, with code: %v, data: %v", fqdn, err, data)
	}

	return nil
}

// CleanUp removes a TXT record used for DNS-01 challenge.
func (y *YandexCloudDNSProvider) CleanUp(fqdn string, value string) error {
	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: y.credentials,
	})

	if err != nil {
		return fmt.Errorf("unable to create SDK: %v", err)
	}

	recordSetRequest := dns.GetDnsZoneRecordSetRequest{
		DnsZoneId: y.dnsZoneId,
		Name:      fqdn,
		Type:      "TXT",
	}

	recordSet, err := sdk.DNS().DnsZone().GetRecordSet(context.Background(), &recordSetRequest)

	if recordSet != nil {
		data := recordSet.Data
		for idx, item := range recordSet.Data {
			if item == value {
				data = append(recordSet.Data[:idx], recordSet.Data[idx+1:]...)
			}
		}

		req := dns.UpdateRecordSetsRequest{
			DnsZoneId: y.dnsZoneId,
			Deletions: []*dns.RecordSet{
				recordSet,
			},
		}

		_, err := sdk.DNS().DnsZone().UpdateRecordSets(context.Background(), &req)

		if err != nil {
			return fmt.Errorf("unable to delete DNS record for: %s, with code: %v", fqdn, err)
		}

		if len(data) > 0 {
			req = dns.UpdateRecordSetsRequest{
				DnsZoneId: y.dnsZoneId,
				Additions: []*dns.RecordSet{
					{
						Name: fqdn,
						Type: "TXT",
						Ttl:  60,
						Data: data,
					},
				},
			}
			_, err := sdk.DNS().DnsZone().UpdateRecordSets(context.Background(), &req)

			if err != nil {
				return fmt.Errorf("unable to create DNS record for: %s, with code: %v, data: %v", fqdn, err, data)
			}
		}
	}

	return nil
}
