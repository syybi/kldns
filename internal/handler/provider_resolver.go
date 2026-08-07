package handler

import services "kldns/internal/service"

func providerResolver() services.DBProviderResolver {
	return services.DBProviderResolver{SecretKey: appSecret()}
}
