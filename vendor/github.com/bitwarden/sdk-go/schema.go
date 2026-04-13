// This file was generated from JSON Schema using quicktype, do not modify it directly.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    clientSettings, err := UnmarshalClientSettings(bytes)
//    bytes, err = clientSettings.Marshal()
//
//    deviceType, err := UnmarshalDeviceType(bytes)
//    bytes, err = deviceType.Marshal()
//
//    command, err := UnmarshalCommand(bytes)
//    bytes, err = command.Marshal()
//
//    passwordLoginRequest, err := UnmarshalPasswordLoginRequest(bytes)
//    bytes, err = passwordLoginRequest.Marshal()
//
//    twoFactorRequest, err := UnmarshalTwoFactorRequest(bytes)
//    bytes, err = twoFactorRequest.Marshal()
//
//    twoFactorProvider, err := UnmarshalTwoFactorProvider(bytes)
//    bytes, err = twoFactorProvider.Marshal()
//
//    kdf, err := UnmarshalKdf(bytes)
//    bytes, err = kdf.Marshal()
//
//    aPIKeyLoginRequest, err := UnmarshalAPIKeyLoginRequest(bytes)
//    bytes, err = aPIKeyLoginRequest.Marshal()
//
//    accessTokenLoginRequest, err := UnmarshalAccessTokenLoginRequest(bytes)
//    bytes, err = accessTokenLoginRequest.Marshal()
//
//    secretVerificationRequest, err := UnmarshalSecretVerificationRequest(bytes)
//    bytes, err = secretVerificationRequest.Marshal()
//
//    fingerprintRequest, err := UnmarshalFingerprintRequest(bytes)
//    bytes, err = fingerprintRequest.Marshal()
//
//    syncRequest, err := UnmarshalSyncRequest(bytes)
//    bytes, err = syncRequest.Marshal()
//
//    secretsCommand, err := UnmarshalSecretsCommand(bytes)
//    bytes, err = secretsCommand.Marshal()
//
//    secretGetRequest, err := UnmarshalSecretGetRequest(bytes)
//    bytes, err = secretGetRequest.Marshal()
//
//    secretsGetRequest, err := UnmarshalSecretsGetRequest(bytes)
//    bytes, err = secretsGetRequest.Marshal()
//
//    secretCreateRequest, err := UnmarshalSecretCreateRequest(bytes)
//    bytes, err = secretCreateRequest.Marshal()
//
//    secretIdentifiersRequest, err := UnmarshalSecretIdentifiersRequest(bytes)
//    bytes, err = secretIdentifiersRequest.Marshal()
//
//    secretPutRequest, err := UnmarshalSecretPutRequest(bytes)
//    bytes, err = secretPutRequest.Marshal()
//
//    secretsDeleteRequest, err := UnmarshalSecretsDeleteRequest(bytes)
//    bytes, err = secretsDeleteRequest.Marshal()
//
//    secretsSyncRequest, err := UnmarshalSecretsSyncRequest(bytes)
//    bytes, err = secretsSyncRequest.Marshal()
//
//    projectsCommand, err := UnmarshalProjectsCommand(bytes)
//    bytes, err = projectsCommand.Marshal()
//
//    projectGetRequest, err := UnmarshalProjectGetRequest(bytes)
//    bytes, err = projectGetRequest.Marshal()
//
//    projectCreateRequest, err := UnmarshalProjectCreateRequest(bytes)
//    bytes, err = projectCreateRequest.Marshal()
//
//    projectsListRequest, err := UnmarshalProjectsListRequest(bytes)
//    bytes, err = projectsListRequest.Marshal()
//
//    projectPutRequest, err := UnmarshalProjectPutRequest(bytes)
//    bytes, err = projectPutRequest.Marshal()
//
//    projectsDeleteRequest, err := UnmarshalProjectsDeleteRequest(bytes)
//    bytes, err = projectsDeleteRequest.Marshal()
//
//    generatorsCommand, err := UnmarshalGeneratorsCommand(bytes)
//    bytes, err = generatorsCommand.Marshal()
//
//    passwordGeneratorRequest, err := UnmarshalPasswordGeneratorRequest(bytes)
//    bytes, err = passwordGeneratorRequest.Marshal()
//
//    responseForAPIKeyLoginResponse, err := UnmarshalResponseForAPIKeyLoginResponse(bytes)
//    bytes, err = responseForAPIKeyLoginResponse.Marshal()
//
//    aPIKeyLoginResponse, err := UnmarshalAPIKeyLoginResponse(bytes)
//    bytes, err = aPIKeyLoginResponse.Marshal()
//
//    twoFactorProviders, err := UnmarshalTwoFactorProviders(bytes)
//    bytes, err = twoFactorProviders.Marshal()
//
//    authenticator, err := UnmarshalAuthenticator(bytes)
//    bytes, err = authenticator.Marshal()
//
//    email, err := UnmarshalEmail(bytes)
//    bytes, err = email.Marshal()
//
//    duo, err := UnmarshalDuo(bytes)
//    bytes, err = duo.Marshal()
//
//    yubiKey, err := UnmarshalYubiKey(bytes)
//    bytes, err = yubiKey.Marshal()
//
//    remember, err := UnmarshalRemember(bytes)
//    bytes, err = remember.Marshal()
//
//    webAuthn, err := UnmarshalWebAuthn(bytes)
//    bytes, err = webAuthn.Marshal()
//
//    responseForPasswordLoginResponse, err := UnmarshalResponseForPasswordLoginResponse(bytes)
//    bytes, err = responseForPasswordLoginResponse.Marshal()
//
//    passwordLoginResponse, err := UnmarshalPasswordLoginResponse(bytes)
//    bytes, err = passwordLoginResponse.Marshal()
//
//    cAPTCHAResponse, err := UnmarshalCAPTCHAResponse(bytes)
//    bytes, err = cAPTCHAResponse.Marshal()
//
//    responseForAccessTokenLoginResponse, err := UnmarshalResponseForAccessTokenLoginResponse(bytes)
//    bytes, err = responseForAccessTokenLoginResponse.Marshal()
//
//    accessTokenLoginResponse, err := UnmarshalAccessTokenLoginResponse(bytes)
//    bytes, err = accessTokenLoginResponse.Marshal()
//
//    responseForSecretIdentifiersResponse, err := UnmarshalResponseForSecretIdentifiersResponse(bytes)
//    bytes, err = responseForSecretIdentifiersResponse.Marshal()
//
//    secretIdentifiersResponse, err := UnmarshalSecretIdentifiersResponse(bytes)
//    bytes, err = secretIdentifiersResponse.Marshal()
//
//    secretIdentifierResponse, err := UnmarshalSecretIdentifierResponse(bytes)
//    bytes, err = secretIdentifierResponse.Marshal()
//
//    responseForSecretResponse, err := UnmarshalResponseForSecretResponse(bytes)
//    bytes, err = responseForSecretResponse.Marshal()
//
//    secretResponse, err := UnmarshalSecretResponse(bytes)
//    bytes, err = secretResponse.Marshal()
//
//    responseForSecretsResponse, err := UnmarshalResponseForSecretsResponse(bytes)
//    bytes, err = responseForSecretsResponse.Marshal()
//
//    secretsResponse, err := UnmarshalSecretsResponse(bytes)
//    bytes, err = secretsResponse.Marshal()
//
//    responseForSecretsDeleteResponse, err := UnmarshalResponseForSecretsDeleteResponse(bytes)
//    bytes, err = responseForSecretsDeleteResponse.Marshal()
//
//    secretsDeleteResponse, err := UnmarshalSecretsDeleteResponse(bytes)
//    bytes, err = secretsDeleteResponse.Marshal()
//
//    secretDeleteResponse, err := UnmarshalSecretDeleteResponse(bytes)
//    bytes, err = secretDeleteResponse.Marshal()
//
//    responseForSecretsSyncResponse, err := UnmarshalResponseForSecretsSyncResponse(bytes)
//    bytes, err = responseForSecretsSyncResponse.Marshal()
//
//    secretsSyncResponse, err := UnmarshalSecretsSyncResponse(bytes)
//    bytes, err = secretsSyncResponse.Marshal()
//
//    responseForProjectResponse, err := UnmarshalResponseForProjectResponse(bytes)
//    bytes, err = responseForProjectResponse.Marshal()
//
//    projectResponse, err := UnmarshalProjectResponse(bytes)
//    bytes, err = projectResponse.Marshal()
//
//    responseForProjectsResponse, err := UnmarshalResponseForProjectsResponse(bytes)
//    bytes, err = responseForProjectsResponse.Marshal()
//
//    projectsResponse, err := UnmarshalProjectsResponse(bytes)
//    bytes, err = projectsResponse.Marshal()
//
//    responseForProjectsDeleteResponse, err := UnmarshalResponseForProjectsDeleteResponse(bytes)
//    bytes, err = responseForProjectsDeleteResponse.Marshal()
//
//    projectsDeleteResponse, err := UnmarshalProjectsDeleteResponse(bytes)
//    bytes, err = projectsDeleteResponse.Marshal()
//
//    projectDeleteResponse, err := UnmarshalProjectDeleteResponse(bytes)
//    bytes, err = projectDeleteResponse.Marshal()
//
//    responseForString, err := UnmarshalResponseForString(bytes)
//    bytes, err = responseForString.Marshal()
//
//    responseForFingerprintResponse, err := UnmarshalResponseForFingerprintResponse(bytes)
//    bytes, err = responseForFingerprintResponse.Marshal()
//
//    fingerprintResponse, err := UnmarshalFingerprintResponse(bytes)
//    bytes, err = fingerprintResponse.Marshal()
//
//    responseForSyncResponse, err := UnmarshalResponseForSyncResponse(bytes)
//    bytes, err = responseForSyncResponse.Marshal()
//
//    syncResponse, err := UnmarshalSyncResponse(bytes)
//    bytes, err = syncResponse.Marshal()
//
//    profileResponse, err := UnmarshalProfileResponse(bytes)
//    bytes, err = profileResponse.Marshal()
//
//    profileOrganizationResponse, err := UnmarshalProfileOrganizationResponse(bytes)
//    bytes, err = profileOrganizationResponse.Marshal()
//
//    folder, err := UnmarshalFolder(bytes)
//    bytes, err = folder.Marshal()
//
//    encString, err := UnmarshalEncString(bytes)
//    bytes, err = encString.Marshal()
//
//    collection, err := UnmarshalCollection(bytes)
//    bytes, err = collection.Marshal()
//
//    cipher, err := UnmarshalCipher(bytes)
//    bytes, err = cipher.Marshal()
//
//    cipherType, err := UnmarshalCipherType(bytes)
//    bytes, err = cipherType.Marshal()
//
//    login, err := UnmarshalLogin(bytes)
//    bytes, err = login.Marshal()
//
//    loginURI, err := UnmarshalLoginURI(bytes)
//    bytes, err = loginURI.Marshal()
//
//    uRIMatchType, err := UnmarshalURIMatchType(bytes)
//    bytes, err = uRIMatchType.Marshal()
//
//    fido2Credential, err := UnmarshalFido2Credential(bytes)
//    bytes, err = fido2Credential.Marshal()
//
//    identity, err := UnmarshalIdentity(bytes)
//    bytes, err = identity.Marshal()
//
//    card, err := UnmarshalCard(bytes)
//    bytes, err = card.Marshal()
//
//    secureNote, err := UnmarshalSecureNote(bytes)
//    bytes, err = secureNote.Marshal()
//
//    secureNoteType, err := UnmarshalSecureNoteType(bytes)
//    bytes, err = secureNoteType.Marshal()
//
//    cipherRepromptType, err := UnmarshalCipherRepromptType(bytes)
//    bytes, err = cipherRepromptType.Marshal()
//
//    localData, err := UnmarshalLocalData(bytes)
//    bytes, err = localData.Marshal()
//
//    attachment, err := UnmarshalAttachment(bytes)
//    bytes, err = attachment.Marshal()
//
//    field, err := UnmarshalField(bytes)
//    bytes, err = field.Marshal()
//
//    fieldType, err := UnmarshalFieldType(bytes)
//    bytes, err = fieldType.Marshal()
//
//    linkedIDType, err := UnmarshalLinkedIDType(bytes)
//    bytes, err = linkedIDType.Marshal()
//
//    loginLinkedIDType, err := UnmarshalLoginLinkedIDType(bytes)
//    bytes, err = loginLinkedIDType.Marshal()
//
//    cardLinkedIDType, err := UnmarshalCardLinkedIDType(bytes)
//    bytes, err = cardLinkedIDType.Marshal()
//
//    identityLinkedIDType, err := UnmarshalIdentityLinkedIDType(bytes)
//    bytes, err = identityLinkedIDType.Marshal()
//
//    passwordHistory, err := UnmarshalPasswordHistory(bytes)
//    bytes, err = passwordHistory.Marshal()
//
//    domainResponse, err := UnmarshalDomainResponse(bytes)
//    bytes, err = domainResponse.Marshal()
//
//    globalDomains, err := UnmarshalGlobalDomains(bytes)
//    bytes, err = globalDomains.Marshal()
//
//    responseForUserAPIKeyResponse, err := UnmarshalResponseForUserAPIKeyResponse(bytes)
//    bytes, err = responseForUserAPIKeyResponse.Marshal()
//
//    userAPIKeyResponse, err := UnmarshalUserAPIKeyResponse(bytes)
//    bytes, err = userAPIKeyResponse.Marshal()

package sdk

import "time"

import "encoding/json"

func UnmarshalClientSettings(data []byte) (ClientSettings, error) {
	var r ClientSettings
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ClientSettings) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalDeviceType(data []byte) (DeviceType, error) {
	var r DeviceType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *DeviceType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCommand(data []byte) (Command, error) {
	var r Command
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Command) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPasswordLoginRequest(data []byte) (PasswordLoginRequest, error) {
	var r PasswordLoginRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PasswordLoginRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTwoFactorRequest(data []byte) (TwoFactorRequest, error) {
	var r TwoFactorRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TwoFactorRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTwoFactorProvider(data []byte) (TwoFactorProvider, error) {
	var r TwoFactorProvider
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TwoFactorProvider) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalKdf(data []byte) (Kdf, error) {
	var r Kdf
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Kdf) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAPIKeyLoginRequest(data []byte) (APIKeyLoginRequest, error) {
	var r APIKeyLoginRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *APIKeyLoginRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAccessTokenLoginRequest(data []byte) (AccessTokenLoginRequest, error) {
	var r AccessTokenLoginRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AccessTokenLoginRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretVerificationRequest(data []byte) (SecretVerificationRequest, error) {
	var r SecretVerificationRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretVerificationRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFingerprintRequest(data []byte) (FingerprintRequest, error) {
	var r FingerprintRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FingerprintRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSyncRequest(data []byte) (SyncRequest, error) {
	var r SyncRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SyncRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretsCommand(data []byte) (SecretsCommand, error) {
	var r SecretsCommand
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretsCommand) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretGetRequest(data []byte) (SecretGetRequest, error) {
	var r SecretGetRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretGetRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretsGetRequest(data []byte) (SecretsGetRequest, error) {
	var r SecretsGetRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretsGetRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretCreateRequest(data []byte) (SecretCreateRequest, error) {
	var r SecretCreateRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretCreateRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretIdentifiersRequest(data []byte) (SecretIdentifiersRequest, error) {
	var r SecretIdentifiersRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretIdentifiersRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretPutRequest(data []byte) (SecretPutRequest, error) {
	var r SecretPutRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretPutRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretsDeleteRequest(data []byte) (SecretsDeleteRequest, error) {
	var r SecretsDeleteRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretsDeleteRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretsSyncRequest(data []byte) (SecretsSyncRequest, error) {
	var r SecretsSyncRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretsSyncRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectsCommand(data []byte) (ProjectsCommand, error) {
	var r ProjectsCommand
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectsCommand) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectGetRequest(data []byte) (ProjectGetRequest, error) {
	var r ProjectGetRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectGetRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectCreateRequest(data []byte) (ProjectCreateRequest, error) {
	var r ProjectCreateRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectCreateRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectsListRequest(data []byte) (ProjectsListRequest, error) {
	var r ProjectsListRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectsListRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectPutRequest(data []byte) (ProjectPutRequest, error) {
	var r ProjectPutRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectPutRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectsDeleteRequest(data []byte) (ProjectsDeleteRequest, error) {
	var r ProjectsDeleteRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectsDeleteRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalGeneratorsCommand(data []byte) (GeneratorsCommand, error) {
	var r GeneratorsCommand
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GeneratorsCommand) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPasswordGeneratorRequest(data []byte) (PasswordGeneratorRequest, error) {
	var r PasswordGeneratorRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PasswordGeneratorRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForAPIKeyLoginResponse(data []byte) (ResponseForAPIKeyLoginResponse, error) {
	var r ResponseForAPIKeyLoginResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForAPIKeyLoginResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAPIKeyLoginResponse(data []byte) (APIKeyLoginResponse, error) {
	var r APIKeyLoginResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *APIKeyLoginResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTwoFactorProviders(data []byte) (TwoFactorProviders, error) {
	var r TwoFactorProviders
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TwoFactorProviders) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAuthenticator(data []byte) (Authenticator, error) {
	var r Authenticator
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Authenticator) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalEmail(data []byte) (Email, error) {
	var r Email
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Email) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalDuo(data []byte) (Duo, error) {
	var r Duo
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Duo) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalYubiKey(data []byte) (YubiKey, error) {
	var r YubiKey
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *YubiKey) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalRemember(data []byte) (Remember, error) {
	var r Remember
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Remember) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalWebAuthn(data []byte) (WebAuthn, error) {
	var r WebAuthn
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *WebAuthn) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForPasswordLoginResponse(data []byte) (ResponseForPasswordLoginResponse, error) {
	var r ResponseForPasswordLoginResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForPasswordLoginResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPasswordLoginResponse(data []byte) (PasswordLoginResponse, error) {
	var r PasswordLoginResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PasswordLoginResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCAPTCHAResponse(data []byte) (CAPTCHAResponse, error) {
	var r CAPTCHAResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CAPTCHAResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForAccessTokenLoginResponse(data []byte) (ResponseForAccessTokenLoginResponse, error) {
	var r ResponseForAccessTokenLoginResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForAccessTokenLoginResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAccessTokenLoginResponse(data []byte) (AccessTokenLoginResponse, error) {
	var r AccessTokenLoginResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AccessTokenLoginResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForSecretIdentifiersResponse(data []byte) (ResponseForSecretIdentifiersResponse, error) {
	var r ResponseForSecretIdentifiersResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForSecretIdentifiersResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretIdentifiersResponse(data []byte) (SecretIdentifiersResponse, error) {
	var r SecretIdentifiersResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretIdentifiersResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretIdentifierResponse(data []byte) (SecretIdentifierResponse, error) {
	var r SecretIdentifierResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretIdentifierResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForSecretResponse(data []byte) (ResponseForSecretResponse, error) {
	var r ResponseForSecretResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForSecretResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretResponse(data []byte) (SecretResponse, error) {
	var r SecretResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForSecretsResponse(data []byte) (ResponseForSecretsResponse, error) {
	var r ResponseForSecretsResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForSecretsResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretsResponse(data []byte) (SecretsResponse, error) {
	var r SecretsResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretsResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForSecretsDeleteResponse(data []byte) (ResponseForSecretsDeleteResponse, error) {
	var r ResponseForSecretsDeleteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForSecretsDeleteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretsDeleteResponse(data []byte) (SecretsDeleteResponse, error) {
	var r SecretsDeleteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretsDeleteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretDeleteResponse(data []byte) (SecretDeleteResponse, error) {
	var r SecretDeleteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretDeleteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForSecretsSyncResponse(data []byte) (ResponseForSecretsSyncResponse, error) {
	var r ResponseForSecretsSyncResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForSecretsSyncResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecretsSyncResponse(data []byte) (SecretsSyncResponse, error) {
	var r SecretsSyncResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecretsSyncResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForProjectResponse(data []byte) (ResponseForProjectResponse, error) {
	var r ResponseForProjectResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForProjectResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectResponse(data []byte) (ProjectResponse, error) {
	var r ProjectResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForProjectsResponse(data []byte) (ResponseForProjectsResponse, error) {
	var r ResponseForProjectsResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForProjectsResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectsResponse(data []byte) (ProjectsResponse, error) {
	var r ProjectsResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectsResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForProjectsDeleteResponse(data []byte) (ResponseForProjectsDeleteResponse, error) {
	var r ResponseForProjectsDeleteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForProjectsDeleteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectsDeleteResponse(data []byte) (ProjectsDeleteResponse, error) {
	var r ProjectsDeleteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectsDeleteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProjectDeleteResponse(data []byte) (ProjectDeleteResponse, error) {
	var r ProjectDeleteResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProjectDeleteResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForString(data []byte) (ResponseForString, error) {
	var r ResponseForString
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForString) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForFingerprintResponse(data []byte) (ResponseForFingerprintResponse, error) {
	var r ResponseForFingerprintResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForFingerprintResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFingerprintResponse(data []byte) (FingerprintResponse, error) {
	var r FingerprintResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FingerprintResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForSyncResponse(data []byte) (ResponseForSyncResponse, error) {
	var r ResponseForSyncResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForSyncResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSyncResponse(data []byte) (SyncResponse, error) {
	var r SyncResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SyncResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProfileResponse(data []byte) (ProfileResponse, error) {
	var r ProfileResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProfileResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalProfileOrganizationResponse(data []byte) (ProfileOrganizationResponse, error) {
	var r ProfileOrganizationResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ProfileOrganizationResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFolder(data []byte) (Folder, error) {
	var r Folder
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Folder) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type EncString string

func UnmarshalEncString(data []byte) (EncString, error) {
	var r EncString
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *EncString) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCollection(data []byte) (Collection, error) {
	var r Collection
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Collection) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCipher(data []byte) (Cipher, error) {
	var r Cipher
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Cipher) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCipherType(data []byte) (CipherType, error) {
	var r CipherType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CipherType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLogin(data []byte) (Login, error) {
	var r Login
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Login) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLoginURI(data []byte) (LoginURI, error) {
	var r LoginURI
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LoginURI) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalURIMatchType(data []byte) (URIMatchType, error) {
	var r URIMatchType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *URIMatchType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFido2Credential(data []byte) (Fido2Credential, error) {
	var r Fido2Credential
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Fido2Credential) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalIdentity(data []byte) (Identity, error) {
	var r Identity
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Identity) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCard(data []byte) (Card, error) {
	var r Card
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Card) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecureNote(data []byte) (SecureNote, error) {
	var r SecureNote
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecureNote) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSecureNoteType(data []byte) (SecureNoteType, error) {
	var r SecureNoteType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SecureNoteType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCipherRepromptType(data []byte) (CipherRepromptType, error) {
	var r CipherRepromptType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CipherRepromptType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLocalData(data []byte) (LocalData, error) {
	var r LocalData
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LocalData) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAttachment(data []byte) (Attachment, error) {
	var r Attachment
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Attachment) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalField(data []byte) (Field, error) {
	var r Field
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Field) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFieldType(data []byte) (FieldType, error) {
	var r FieldType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *FieldType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLinkedIDType(data []byte) (LinkedIDType, error) {
	var r LinkedIDType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LinkedIDType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLoginLinkedIDType(data []byte) (LoginLinkedIDType, error) {
	var r LoginLinkedIDType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LoginLinkedIDType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalCardLinkedIDType(data []byte) (CardLinkedIDType, error) {
	var r CardLinkedIDType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *CardLinkedIDType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalIdentityLinkedIDType(data []byte) (IdentityLinkedIDType, error) {
	var r IdentityLinkedIDType
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *IdentityLinkedIDType) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPasswordHistory(data []byte) (PasswordHistory, error) {
	var r PasswordHistory
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PasswordHistory) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalDomainResponse(data []byte) (DomainResponse, error) {
	var r DomainResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *DomainResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalGlobalDomains(data []byte) (GlobalDomains, error) {
	var r GlobalDomains
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GlobalDomains) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalResponseForUserAPIKeyResponse(data []byte) (ResponseForUserAPIKeyResponse, error) {
	var r ResponseForUserAPIKeyResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ResponseForUserAPIKeyResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalUserAPIKeyResponse(data []byte) (UserAPIKeyResponse, error) {
	var r UserAPIKeyResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *UserAPIKeyResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Basic client behavior settings. These settings specify the various targets and behavior
// of the Bitwarden Client. They are optional and uneditable once the client is
// initialized.
//
// Defaults to
//
// ``` # use bitwarden_core::{ClientSettings, DeviceType}; let settings = ClientSettings {
// identity_url: "https://identity.bitwarden.com".to_string(), api_url:
// "https://api.bitwarden.com".to_string(), user_agent: "Bitwarden Rust-SDK".to_string(),
// device_type: DeviceType::SDK, }; let default = ClientSettings::default(); ```
type ClientSettings struct {
	// The api url of the targeted Bitwarden instance. Defaults to `https://api.bitwarden.com`            
	APIURL                                                                                    *string     `json:"apiUrl,omitempty"`
	// Device type to send to Bitwarden. Defaults to SDK                                                  
	DeviceType                                                                                *DeviceType `json:"deviceType,omitempty"`
	// The identity url of the targeted Bitwarden instance. Defaults to                                   
	// `https://identity.bitwarden.com`                                                                   
	IdentityURL                                                                               *string     `json:"identityUrl,omitempty"`
	// The user_agent to sent to Bitwarden. Defaults to `Bitwarden Rust-SDK`                              
	UserAgent                                                                                 *string     `json:"userAgent,omitempty"`
}

// Login with username and password
//
// This command is for initiating an authentication handshake with Bitwarden. Authorization
// may fail due to requiring 2fa or captcha challenge completion despite accurate
// credentials.
//
// This command is not capable of handling authentication requiring 2fa or captcha.
//
// Returns: [PasswordLoginResponse](bitwarden::auth::login::PasswordLoginResponse)
//
// Login with API Key
//
// This command is for initiating an authentication handshake with Bitwarden.
//
// Returns: [ApiKeyLoginResponse](bitwarden::auth::login::ApiKeyLoginResponse)
//
// Login with Secrets Manager Access Token
//
// This command is for initiating an authentication handshake with Bitwarden.
//
// Returns: [ApiKeyLoginResponse](bitwarden::auth::login::ApiKeyLoginResponse)
//
// > Requires Authentication Get the API key of the currently authenticated user
//
// Returns: [UserApiKeyResponse](bitwarden::platform::UserApiKeyResponse)
//
// Get the user's passphrase
//
// Returns: String
//
// > Requires Authentication Retrieve all user data, ciphers and organizations the user is a
// part of
//
// Returns: [SyncResponse](bitwarden::vault::SyncResponse)
type Command struct {
	PasswordLogin    *PasswordLoginRequest      `json:"passwordLogin,omitempty"`
	APIKeyLogin      *APIKeyLoginRequest        `json:"apiKeyLogin,omitempty"`
	LoginAccessToken *AccessTokenLoginRequest   `json:"loginAccessToken,omitempty"`
	GetUserAPIKey    *SecretVerificationRequest `json:"getUserApiKey,omitempty"`
	Fingerprint      *FingerprintRequest        `json:"fingerprint,omitempty"`
	Sync             *SyncRequest               `json:"sync,omitempty"`
	Secrets          *SecretsCommand            `json:"secrets,omitempty"`
	Projects         *ProjectsCommand           `json:"projects,omitempty"`
	Generators       *GeneratorsCommand         `json:"generators,omitempty"`
}

// Login to Bitwarden with Api Key
type APIKeyLoginRequest struct {
	// Bitwarden account client_id             
	ClientID                            string `json:"clientId"`
	// Bitwarden account client_secret         
	ClientSecret                        string `json:"clientSecret"`
	// Bitwarden account master password       
	Password                            string `json:"password"`
}

type FingerprintRequest struct {
	// The input material, used in the fingerprint generation process.       
	FingerprintMaterial                                               string `json:"fingerprintMaterial"`
	// The user's public key encoded with base64.                            
	PublicKey                                                         string `json:"publicKey"`
}

// Generate a password
//
// Returns: [String]
type GeneratorsCommand struct {
	GeneratePassword PasswordGeneratorRequest `json:"generatePassword"`
}

// Password generator request options.
type PasswordGeneratorRequest struct {
	// When set to true, the generated password will not contain ambiguous characters. The             
	// ambiguous characters are: I, O, l, 0, 1                                                         
	AvoidAmbiguous                                                                              bool   `json:"avoidAmbiguous"`
	// The length of the generated password. Note that the password length must be greater than        
	// the sum of all the minimums.                                                                    
	Length                                                                                      int64  `json:"length"`
	// Include lowercase characters (a-z).                                                             
	Lowercase                                                                                   bool   `json:"lowercase"`
	// The minimum number of lowercase characters in the generated password. When set, the value       
	// must be between 1 and 9. This value is ignored if lowercase is false.                           
	MinLowercase                                                                                *int64 `json:"minLowercase,omitempty"`
	// The minimum number of numbers in the generated password. When set, the value must be            
	// between 1 and 9. This value is ignored if numbers is false.                                     
	MinNumber                                                                                   *int64 `json:"minNumber,omitempty"`
	// The minimum number of special characters in the generated password. When set, the value         
	// must be between 1 and 9. This value is ignored if special is false.                             
	MinSpecial                                                                                  *int64 `json:"minSpecial,omitempty"`
	// The minimum number of uppercase characters in the generated password. When set, the value       
	// must be between 1 and 9. This value is ignored if uppercase is false.                           
	MinUppercase                                                                                *int64 `json:"minUppercase,omitempty"`
	// Include numbers (0-9).                                                                          
	Numbers                                                                                     bool   `json:"numbers"`
	// Include special characters: ! @ # $ % ^ & *                                                     
	Special                                                                                     bool   `json:"special"`
	// Include uppercase characters (A-Z).                                                             
	Uppercase                                                                                   bool   `json:"uppercase"`
}

type SecretVerificationRequest struct {
	// The user's master password to use for user verification. If supplied, this will be used        
	// for verification purposes.                                                                     
	MasterPassword                                                                            *string `json:"masterPassword,omitempty"`
	// Alternate user verification method through OTP. This is provided for users who have no         
	// master password due to use of Customer Managed Encryption. Must be present and valid if        
	// master_password is absent.                                                                     
	Otp                                                                                       *string `json:"otp,omitempty"`
}

// Login to Bitwarden with access token
type AccessTokenLoginRequest struct {
	// Bitwarden service API access token        
	AccessToken                          string  `json:"accessToken"`
	StateFile                            *string `json:"stateFile,omitempty"`
}

// Login to Bitwarden with Username and Password
type PasswordLoginRequest struct {
	// Bitwarden account email address                    
	Email                               string            `json:"email"`
	// Kdf from prelogin                                  
	Kdf                                 Kdf               `json:"kdf"`
	// Bitwarden account master password                  
	Password                            string            `json:"password"`
	TwoFactor                           *TwoFactorRequest `json:"twoFactor,omitempty"`
}

// Kdf from prelogin
//
// Key Derivation Function for Bitwarden Account
//
// In Bitwarden accounts can use multiple KDFs to derive their master key from their
// password. This Enum represents all the possible KDFs.
type Kdf struct {
	PBKDF2   *PBKDF2   `json:"pBKDF2,omitempty"`
	Argon2ID *Argon2ID `json:"argon2id,omitempty"`
}

type Argon2ID struct {
	Iterations  int64 `json:"iterations"`
	Memory      int64 `json:"memory"`
	Parallelism int64 `json:"parallelism"`
}

type PBKDF2 struct {
	Iterations int64 `json:"iterations"`
}

type TwoFactorRequest struct {
	// Two-factor provider                  
	Provider              TwoFactorProvider `json:"provider"`
	// Two-factor remember                  
	Remember              bool              `json:"remember"`
	// Two-factor Token                     
	Token                 string            `json:"token"`
}

// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Retrieve a project by the provided identifier
//
// Returns: [ProjectResponse](bitwarden::secrets_manager::projects::ProjectResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Creates a new project in the provided organization using the given data
//
// Returns: [ProjectResponse](bitwarden::secrets_manager::projects::ProjectResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Lists all projects of the given organization
//
// Returns: [ProjectsResponse](bitwarden::secrets_manager::projects::ProjectsResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Updates an existing project with the provided ID using the given data
//
// Returns: [ProjectResponse](bitwarden::secrets_manager::projects::ProjectResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Deletes all the projects whose IDs match the provided ones
//
// Returns:
// [ProjectsDeleteResponse](bitwarden::secrets_manager::projects::ProjectsDeleteResponse)
type ProjectsCommand struct {
	Get    *ProjectGetRequest     `json:"get,omitempty"`
	Create *ProjectCreateRequest  `json:"create,omitempty"`
	List   *ProjectsListRequest   `json:"list,omitempty"`
	Update *ProjectPutRequest     `json:"update,omitempty"`
	Delete *ProjectsDeleteRequest `json:"delete,omitempty"`
}

type ProjectCreateRequest struct {
	Name                                             string `json:"name"`
	// Organization where the project will be created       
	OrganizationID                                   string `json:"organizationId"`
}

type ProjectsDeleteRequest struct {
	// IDs of the projects to delete         
	IDS                             []string `json:"ids"`
}

type ProjectGetRequest struct {
	// ID of the project to retrieve       
	ID                              string `json:"id"`
}

type ProjectsListRequest struct {
	// Organization to retrieve all the projects from       
	OrganizationID                                   string `json:"organizationId"`
}

type ProjectPutRequest struct {
	// ID of the project to modify                    
	ID                                         string `json:"id"`
	Name                                       string `json:"name"`
	// Organization ID of the project to modify       
	OrganizationID                             string `json:"organizationId"`
}

// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Retrieve a secret by the provided identifier
//
// Returns: [SecretResponse](bitwarden::secrets_manager::secrets::SecretResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Retrieve secrets by the provided identifiers
//
// Returns: [SecretsResponse](bitwarden::secrets_manager::secrets::SecretsResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Creates a new secret in the provided organization using the given data
//
// Returns: [SecretResponse](bitwarden::secrets_manager::secrets::SecretResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Lists all secret identifiers of the given organization, to then retrieve each
// secret, use `CreateSecret`
//
// Returns:
// [SecretIdentifiersResponse](bitwarden::secrets_manager::secrets::SecretIdentifiersResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Updates an existing secret with the provided ID using the given data
//
// Returns: [SecretResponse](bitwarden::secrets_manager::secrets::SecretResponse)
//
// > Requires Authentication > Requires using an Access Token for login or calling Sync at
// least once Deletes all the secrets whose IDs match the provided ones
//
// Returns:
// [SecretsDeleteResponse](bitwarden::secrets_manager::secrets::SecretsDeleteResponse)
//
// > Requires Authentication > Requires using an Access Token for login Retrieve the secrets
// accessible by the authenticated machine account Optionally, provide the last synced date
// to assess whether any changes have occurred If changes are detected, retrieves all the
// secrets accessible by the authenticated machine account
//
// Returns: [SecretsSyncResponse](bitwarden::secrets_manager::secrets::SecretsSyncResponse)
type SecretsCommand struct {
	Get      *SecretGetRequest         `json:"get,omitempty"`
	GetByIDS *SecretsGetRequest        `json:"getByIds,omitempty"`
	Create   *SecretCreateRequest      `json:"create,omitempty"`
	List     *SecretIdentifiersRequest `json:"list,omitempty"`
	Update   *SecretPutRequest         `json:"update,omitempty"`
	Delete   *SecretsDeleteRequest     `json:"delete,omitempty"`
	Sync     *SecretsSyncRequest       `json:"sync,omitempty"`
}

type SecretCreateRequest struct {
	Key                                                   string   `json:"key"`
	Note                                                  string   `json:"note"`
	// Organization where the secret will be created               
	OrganizationID                                        string   `json:"organizationId"`
	// IDs of the projects that this secret will belong to         
	ProjectIDS                                            []string `json:"projectIds,omitempty"`
	Value                                                 string   `json:"value"`
}

type SecretsDeleteRequest struct {
	// IDs of the secrets to delete         
	IDS                            []string `json:"ids"`
}

type SecretGetRequest struct {
	// ID of the secret to retrieve       
	ID                             string `json:"id"`
}

type SecretsGetRequest struct {
	// IDs of the secrets to retrieve         
	IDS                              []string `json:"ids"`
}

type SecretIdentifiersRequest struct {
	// Organization to retrieve all the secrets from       
	OrganizationID                                  string `json:"organizationId"`
}

type SecretsSyncRequest struct {
	// Optional date time a sync last occurred           
	LastSyncedDate                            *time.Time `json:"lastSyncedDate,omitempty"`
	// Organization to sync secrets from                 
	OrganizationID                            string     `json:"organizationId"`
}

type SecretPutRequest struct {
	// ID of the secret to modify                      
	ID                                        string   `json:"id"`
	Key                                       string   `json:"key"`
	Note                                      string   `json:"note"`
	// Organization ID of the secret to modify         
	OrganizationID                            string   `json:"organizationId"`
	ProjectIDS                                []string `json:"projectIds,omitempty"`
	Value                                     string   `json:"value"`
}

type SyncRequest struct {
	// Exclude the subdomains from the response, defaults to false      
	ExcludeSubdomains                                             *bool `json:"excludeSubdomains,omitempty"`
}

type ResponseForAPIKeyLoginResponse struct {
	// The response data. Populated if `success` is true.                                           
	Data                                                                       *APIKeyLoginResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                     
	ErrorMessage                                                               *string              `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                    
	Success                                                                    bool                 `json:"success"`
}

type APIKeyLoginResponse struct {
	Authenticated                                                         bool                `json:"authenticated"`
	// Whether or not the user is required to update their master password                    
	ForcePasswordReset                                                    bool                `json:"forcePasswordReset"`
	// TODO: What does this do?                                                               
	ResetMasterPassword                                                   bool                `json:"resetMasterPassword"`
	TwoFactor                                                             *TwoFactorProviders `json:"twoFactor,omitempty"`
}

type TwoFactorProviders struct {
	Authenticator                                                         *Authenticator `json:"authenticator,omitempty"`
	// Duo-backed 2fa                                                                    
	Duo                                                                   *Duo           `json:"duo,omitempty"`
	// Email 2fa                                                                         
	Email                                                                 *Email         `json:"email,omitempty"`
	// Duo-backed 2fa operated by an organization the user is a member of                
	OrganizationDuo                                                       *Duo           `json:"organizationDuo,omitempty"`
	// Presence indicates the user has stored this device as bypassing 2fa               
	Remember                                                              *Remember      `json:"remember,omitempty"`
	// WebAuthn-backed 2fa                                                               
	WebAuthn                                                              *WebAuthn      `json:"webAuthn,omitempty"`
	// Yubikey-backed 2fa                                                                
	YubiKey                                                               *YubiKey       `json:"yubiKey,omitempty"`
}

type Authenticator struct {
}

type Duo struct {
	Host      string `json:"host"`
	Signature string `json:"signature"`
}

type Email struct {
	// The email to request a 2fa TOTP for       
	Email                                 string `json:"email"`
}

type Remember struct {
}

type WebAuthn struct {
}

type YubiKey struct {
	// Whether the stored yubikey supports near field communication     
	NFC                                                            bool `json:"nfc"`
}

type ResponseForPasswordLoginResponse struct {
	// The response data. Populated if `success` is true.                                             
	Data                                                                       *PasswordLoginResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                       
	ErrorMessage                                                               *string                `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                      
	Success                                                                    bool                   `json:"success"`
}

type PasswordLoginResponse struct {
	Authenticated                                                                              bool                `json:"authenticated"`
	// The information required to present the user with a captcha challenge. Only present when                    
	// authentication fails due to requiring validation of a captcha challenge.                                    
	CAPTCHA                                                                                    *CAPTCHAResponse    `json:"captcha,omitempty"`
	// Whether or not the user is required to update their master password                                         
	ForcePasswordReset                                                                         bool                `json:"forcePasswordReset"`
	// TODO: What does this do?                                                                                    
	ResetMasterPassword                                                                        bool                `json:"resetMasterPassword"`
	// The available two factor authentication options. Present only when authentication fails                     
	// due to requiring a second authentication factor.                                                            
	TwoFactor                                                                                  *TwoFactorProviders `json:"twoFactor,omitempty"`
}

type CAPTCHAResponse struct {
	// hcaptcha site key       
	SiteKey             string `json:"siteKey"`
}

type ResponseForAccessTokenLoginResponse struct {
	// The response data. Populated if `success` is true.                                                
	Data                                                                       *AccessTokenLoginResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                          
	ErrorMessage                                                               *string                   `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                         
	Success                                                                    bool                      `json:"success"`
}

type AccessTokenLoginResponse struct {
	Authenticated                                                         bool                `json:"authenticated"`
	// Whether or not the user is required to update their master password                    
	ForcePasswordReset                                                    bool                `json:"forcePasswordReset"`
	// TODO: What does this do?                                                               
	ResetMasterPassword                                                   bool                `json:"resetMasterPassword"`
	TwoFactor                                                             *TwoFactorProviders `json:"twoFactor,omitempty"`
}

type ResponseForSecretIdentifiersResponse struct {
	// The response data. Populated if `success` is true.                                                 
	Data                                                                       *SecretIdentifiersResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                           
	ErrorMessage                                                               *string                    `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                          
	Success                                                                    bool                       `json:"success"`
}

type SecretIdentifiersResponse struct {
	Data []SecretIdentifierResponse `json:"data"`
}

type SecretIdentifierResponse struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	OrganizationID string `json:"organizationId"`
}

type ResponseForSecretResponse struct {
	// The response data. Populated if `success` is true.                                      
	Data                                                                       *SecretResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                
	ErrorMessage                                                               *string         `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                               
	Success                                                                    bool            `json:"success"`
}

type SecretResponse struct {
	CreationDate   time.Time `json:"creationDate"`
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	Note           string    `json:"note"`
	OrganizationID string    `json:"organizationId"`
	ProjectID      *string   `json:"projectId,omitempty"`
	RevisionDate   time.Time `json:"revisionDate"`
	Value          string    `json:"value"`
}

type ResponseForSecretsResponse struct {
	// The response data. Populated if `success` is true.                                       
	Data                                                                       *SecretsResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                 
	ErrorMessage                                                               *string          `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                
	Success                                                                    bool             `json:"success"`
}

type SecretsResponse struct {
	Data []SecretResponse `json:"data"`
}

type ResponseForSecretsDeleteResponse struct {
	// The response data. Populated if `success` is true.                                             
	Data                                                                       *SecretsDeleteResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                       
	ErrorMessage                                                               *string                `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                      
	Success                                                                    bool                   `json:"success"`
}

type SecretsDeleteResponse struct {
	Data []SecretDeleteResponse `json:"data"`
}

type SecretDeleteResponse struct {
	Error *string `json:"error,omitempty"`
	ID    string  `json:"id"`
}

type ResponseForSecretsSyncResponse struct {
	// The response data. Populated if `success` is true.                                           
	Data                                                                       *SecretsSyncResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                     
	ErrorMessage                                                               *string              `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                    
	Success                                                                    bool                 `json:"success"`
}

type SecretsSyncResponse struct {
	HasChanges bool             `json:"hasChanges"`
	Secrets    []SecretResponse `json:"secrets,omitempty"`
}

type ResponseForProjectResponse struct {
	// The response data. Populated if `success` is true.                                       
	Data                                                                       *ProjectResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                 
	ErrorMessage                                                               *string          `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                
	Success                                                                    bool             `json:"success"`
}

type ProjectResponse struct {
	CreationDate   time.Time `json:"creationDate"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	OrganizationID string    `json:"organizationId"`
	RevisionDate   time.Time `json:"revisionDate"`
}

type ResponseForProjectsResponse struct {
	// The response data. Populated if `success` is true.                                        
	Data                                                                       *ProjectsResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                  
	ErrorMessage                                                               *string           `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                 
	Success                                                                    bool              `json:"success"`
}

type ProjectsResponse struct {
	Data []ProjectResponse `json:"data"`
}

type ResponseForProjectsDeleteResponse struct {
	// The response data. Populated if `success` is true.                                              
	Data                                                                       *ProjectsDeleteResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                        
	ErrorMessage                                                               *string                 `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                       
	Success                                                                    bool                    `json:"success"`
}

type ProjectsDeleteResponse struct {
	Data []ProjectDeleteResponse `json:"data"`
}

type ProjectDeleteResponse struct {
	Error *string `json:"error,omitempty"`
	ID    string  `json:"id"`
}

type ResponseForString struct {
	// The response data. Populated if `success` is true.                              
	Data                                                                       *string `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.        
	ErrorMessage                                                               *string `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                       
	Success                                                                    bool    `json:"success"`
}

type ResponseForFingerprintResponse struct {
	// The response data. Populated if `success` is true.                                           
	Data                                                                       *FingerprintResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                     
	ErrorMessage                                                               *string              `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                    
	Success                                                                    bool                 `json:"success"`
}

type FingerprintResponse struct {
	Fingerprint string `json:"fingerprint"`
}

type ResponseForSyncResponse struct {
	// The response data. Populated if `success` is true.                                    
	Data                                                                       *SyncResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.              
	ErrorMessage                                                               *string       `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                             
	Success                                                                    bool          `json:"success"`
}

type SyncResponse struct {
	// List of ciphers accessible by the user                                                               
	Ciphers                                                                                 []Cipher        `json:"ciphers"`
	Collections                                                                             []Collection    `json:"collections"`
	Domains                                                                                 *DomainResponse `json:"domains,omitempty"`
	Folders                                                                                 []Folder        `json:"folders"`
	// Data about the user, including their encryption keys and the organizations they are a                
	// part of                                                                                              
	Profile                                                                                 ProfileResponse `json:"profile"`
}

type Cipher struct {
	Attachments                                                                              []Attachment       `json:"attachments,omitempty"`
	Card                                                                                     *Card              `json:"card,omitempty"`
	CollectionIDS                                                                            []string           `json:"collectionIds"`
	CreationDate                                                                             time.Time          `json:"creationDate"`
	DeletedDate                                                                              *time.Time         `json:"deletedDate,omitempty"`
	Edit                                                                                     bool               `json:"edit"`
	Favorite                                                                                 bool               `json:"favorite"`
	Fields                                                                                   []Field            `json:"fields,omitempty"`
	FolderID                                                                                 *string            `json:"folderId,omitempty"`
	ID                                                                                       *string            `json:"id,omitempty"`
	Identity                                                                                 *Identity          `json:"identity,omitempty"`
	// More recent ciphers uses individual encryption keys to encrypt the other fields of the                   
	// Cipher.                                                                                                  
	Key                                                                                      *string            `json:"key,omitempty"`
	LocalData                                                                                *LocalData         `json:"localData,omitempty"`
	Login                                                                                    *Login             `json:"login,omitempty"`
	Name                                                                                     string             `json:"name"`
	Notes                                                                                    *string            `json:"notes,omitempty"`
	OrganizationID                                                                           *string            `json:"organizationId,omitempty"`
	OrganizationUseTotp                                                                      bool               `json:"organizationUseTotp"`
	PasswordHistory                                                                          []PasswordHistory  `json:"passwordHistory,omitempty"`
	Reprompt                                                                                 CipherRepromptType `json:"reprompt"`
	RevisionDate                                                                             time.Time          `json:"revisionDate"`
	SecureNote                                                                               *SecureNote        `json:"secureNote,omitempty"`
	Type                                                                                     CipherType         `json:"type"`
	ViewPassword                                                                             bool               `json:"viewPassword"`
}

type Attachment struct {
	FileName                                   *string `json:"fileName,omitempty"`
	ID                                         *string `json:"id,omitempty"`
	Key                                        *string `json:"key,omitempty"`
	Size                                       *string `json:"size,omitempty"`
	// Readable size, ex: "4.2 KB" or "1.43 GB"        
	SizeName                                   *string `json:"sizeName,omitempty"`
	URL                                        *string `json:"url,omitempty"`
}

type Card struct {
	Brand          *string `json:"brand,omitempty"`
	CardholderName *string `json:"cardholderName,omitempty"`
	Code           *string `json:"code,omitempty"`
	ExpMonth       *string `json:"expMonth,omitempty"`
	ExpYear        *string `json:"expYear,omitempty"`
	Number         *string `json:"number,omitempty"`
}

type Field struct {
	LinkedID *LinkedIDType `json:"linkedId,omitempty"`
	Name     *string       `json:"name,omitempty"`
	Type     FieldType     `json:"type"`
	Value    *string       `json:"value,omitempty"`
}

type Identity struct {
	Address1       *string `json:"address1,omitempty"`
	Address2       *string `json:"address2,omitempty"`
	Address3       *string `json:"address3,omitempty"`
	City           *string `json:"city,omitempty"`
	Company        *string `json:"company,omitempty"`
	Country        *string `json:"country,omitempty"`
	Email          *string `json:"email,omitempty"`
	FirstName      *string `json:"firstName,omitempty"`
	LastName       *string `json:"lastName,omitempty"`
	LicenseNumber  *string `json:"licenseNumber,omitempty"`
	MiddleName     *string `json:"middleName,omitempty"`
	PassportNumber *string `json:"passportNumber,omitempty"`
	Phone          *string `json:"phone,omitempty"`
	PostalCode     *string `json:"postalCode,omitempty"`
	Ssn            *string `json:"ssn,omitempty"`
	State          *string `json:"state,omitempty"`
	Title          *string `json:"title,omitempty"`
	Username       *string `json:"username,omitempty"`
}

type LocalData struct {
	LastLaunched *int64 `json:"lastLaunched,omitempty"`
	LastUsedDate *int64 `json:"lastUsedDate,omitempty"`
}

type Login struct {
	AutofillOnPageLoad   *bool             `json:"autofillOnPageLoad,omitempty"`
	Fido2Credentials     []Fido2Credential `json:"fido2Credentials,omitempty"`
	Password             *string           `json:"password,omitempty"`
	PasswordRevisionDate *time.Time        `json:"passwordRevisionDate,omitempty"`
	Totp                 *string           `json:"totp,omitempty"`
	Uris                 []LoginURI        `json:"uris,omitempty"`
	Username             *string           `json:"username,omitempty"`
}

type Fido2Credential struct {
	Counter         string    `json:"counter"`
	CreationDate    time.Time `json:"creationDate"`
	CredentialID    string    `json:"credentialId"`
	Discoverable    string    `json:"discoverable"`
	KeyAlgorithm    string    `json:"keyAlgorithm"`
	KeyCurve        string    `json:"keyCurve"`
	KeyType         string    `json:"keyType"`
	KeyValue        string    `json:"keyValue"`
	RpID            string    `json:"rpId"`
	RpName          *string   `json:"rpName,omitempty"`
	UserDisplayName *string   `json:"userDisplayName,omitempty"`
	UserHandle      *string   `json:"userHandle,omitempty"`
	UserName        *string   `json:"userName,omitempty"`
}

type LoginURI struct {
	Match       *URIMatchType `json:"match,omitempty"`
	URI         *string       `json:"uri,omitempty"`
	URIChecksum *string       `json:"uriChecksum,omitempty"`
}

type PasswordHistory struct {
	LastUsedDate time.Time `json:"lastUsedDate"`
	Password     string    `json:"password"`
}

type SecureNote struct {
	Type SecureNoteType `json:"type"`
}

type Collection struct {
	ExternalID     *string `json:"externalId,omitempty"`
	HidePasswords  bool    `json:"hidePasswords"`
	ID             *string `json:"id,omitempty"`
	Name           string  `json:"name"`
	OrganizationID string  `json:"organizationId"`
	ReadOnly       bool    `json:"readOnly"`
}

type DomainResponse struct {
	EquivalentDomains       [][]string      `json:"equivalentDomains"`
	GlobalEquivalentDomains []GlobalDomains `json:"globalEquivalentDomains"`
}

type GlobalDomains struct {
	Domains  []string `json:"domains"`
	Excluded bool     `json:"excluded"`
	Type     int64    `json:"type"`
}

type Folder struct {
	ID           *string   `json:"id,omitempty"`
	Name         string    `json:"name"`
	RevisionDate time.Time `json:"revisionDate"`
}

// Data about the user, including their encryption keys and the organizations they are a
// part of
type ProfileResponse struct {
	Email         string                        `json:"email"`
	ID            string                        `json:"id"`
	Name          string                        `json:"name"`
	Organizations []ProfileOrganizationResponse `json:"organizations"`
}

type ProfileOrganizationResponse struct {
	ID string `json:"id"`
}

type ResponseForUserAPIKeyResponse struct {
	// The response data. Populated if `success` is true.                                          
	Data                                                                       *UserAPIKeyResponse `json:"data,omitempty"`
	// A message for any error that may occur. Populated if `success` is false.                    
	ErrorMessage                                                               *string             `json:"errorMessage,omitempty"`
	// Whether or not the SDK request succeeded.                                                   
	Success                                                                    bool                `json:"success"`
}

type UserAPIKeyResponse struct {
	// The user's API key, which represents the client_secret portion of an oauth request.       
	APIKey                                                                                string `json:"apiKey"`
}

// Device type to send to Bitwarden. Defaults to SDK
type DeviceType string

const (
	Android          DeviceType = "Android"
	AndroidAmazon    DeviceType = "AndroidAmazon"
	ChromeBrowser    DeviceType = "ChromeBrowser"
	ChromeExtension  DeviceType = "ChromeExtension"
	EdgeBrowser      DeviceType = "EdgeBrowser"
	EdgeExtension    DeviceType = "EdgeExtension"
	FirefoxBrowser   DeviceType = "FirefoxBrowser"
	FirefoxExtension DeviceType = "FirefoxExtension"
	IEBrowser        DeviceType = "IEBrowser"
	IOS              DeviceType = "iOS"
	LinuxCLI         DeviceType = "LinuxCLI"
	LinuxDesktop     DeviceType = "LinuxDesktop"
	MACOSCLI         DeviceType = "MacOsCLI"
	MACOSDesktop     DeviceType = "MacOsDesktop"
	OperaBrowser     DeviceType = "OperaBrowser"
	OperaExtension   DeviceType = "OperaExtension"
	SDK              DeviceType = "SDK"
	SafariBrowser    DeviceType = "SafariBrowser"
	SafariExtension  DeviceType = "SafariExtension"
	Server           DeviceType = "Server"
	UWP              DeviceType = "UWP"
	UnknownBrowser   DeviceType = "UnknownBrowser"
	VivaldiBrowser   DeviceType = "VivaldiBrowser"
	VivaldiExtension DeviceType = "VivaldiExtension"
	WindowsCLI       DeviceType = "WindowsCLI"
	WindowsDesktop   DeviceType = "WindowsDesktop"
)

// Two-factor provider
type TwoFactorProvider string

const (
	OrganizationDuo                TwoFactorProvider = "OrganizationDuo"
	TwoFactorProviderAuthenticator TwoFactorProvider = "Authenticator"
	TwoFactorProviderDuo           TwoFactorProvider = "Duo"
	TwoFactorProviderEmail         TwoFactorProvider = "Email"
	TwoFactorProviderRemember      TwoFactorProvider = "Remember"
	TwoFactorProviderWebAuthn      TwoFactorProvider = "WebAuthn"
	U2F                            TwoFactorProvider = "U2f"
	Yubikey                        TwoFactorProvider = "Yubikey"
)

type LinkedIDType string

const (
	LinkedIDTypeAddress1       LinkedIDType = "Address1"
	LinkedIDTypeAddress2       LinkedIDType = "Address2"
	LinkedIDTypeAddress3       LinkedIDType = "Address3"
	LinkedIDTypeBrand          LinkedIDType = "Brand"
	LinkedIDTypeCardholderName LinkedIDType = "CardholderName"
	LinkedIDTypeCity           LinkedIDType = "City"
	LinkedIDTypeCode           LinkedIDType = "Code"
	LinkedIDTypeCompany        LinkedIDType = "Company"
	LinkedIDTypeCountry        LinkedIDType = "Country"
	LinkedIDTypeEmail          LinkedIDType = "Email"
	LinkedIDTypeExpMonth       LinkedIDType = "ExpMonth"
	LinkedIDTypeExpYear        LinkedIDType = "ExpYear"
	LinkedIDTypeFirstName      LinkedIDType = "FirstName"
	LinkedIDTypeFullName       LinkedIDType = "FullName"
	LinkedIDTypeLastName       LinkedIDType = "LastName"
	LinkedIDTypeLicenseNumber  LinkedIDType = "LicenseNumber"
	LinkedIDTypeMiddleName     LinkedIDType = "MiddleName"
	LinkedIDTypeNumber         LinkedIDType = "Number"
	LinkedIDTypePassportNumber LinkedIDType = "PassportNumber"
	LinkedIDTypePassword       LinkedIDType = "Password"
	LinkedIDTypePhone          LinkedIDType = "Phone"
	LinkedIDTypePostalCode     LinkedIDType = "PostalCode"
	LinkedIDTypeSsn            LinkedIDType = "Ssn"
	LinkedIDTypeState          LinkedIDType = "State"
	LinkedIDTypeTitle          LinkedIDType = "Title"
	LinkedIDTypeUsername       LinkedIDType = "Username"
)

type FieldType string

const (
	Boolean FieldType = "Boolean"
	Hidden  FieldType = "Hidden"
	Linked  FieldType = "Linked"
	Text    FieldType = "Text"
)

type URIMatchType string

const (
	Domain            URIMatchType = "domain"
	Exact             URIMatchType = "exact"
	Host              URIMatchType = "host"
	Never             URIMatchType = "never"
	RegularExpression URIMatchType = "regularExpression"
	StartsWith        URIMatchType = "startsWith"
)

type CipherRepromptType string

const (
	CipherRepromptTypePassword CipherRepromptType = "Password"
	None                       CipherRepromptType = "None"
)

type SecureNoteType string

const (
	Generic SecureNoteType = "Generic"
)

type CipherType string

const (
	CipherTypeCard       CipherType = "Card"
	CipherTypeIdentity   CipherType = "Identity"
	CipherTypeLogin      CipherType = "Login"
	CipherTypeSecureNote CipherType = "SecureNote"
)

type LoginLinkedIDType string

const (
	LoginLinkedIDTypePassword LoginLinkedIDType = "Password"
	LoginLinkedIDTypeUsername LoginLinkedIDType = "Username"
)

type CardLinkedIDType string

const (
	CardLinkedIDTypeBrand          CardLinkedIDType = "Brand"
	CardLinkedIDTypeCardholderName CardLinkedIDType = "CardholderName"
	CardLinkedIDTypeCode           CardLinkedIDType = "Code"
	CardLinkedIDTypeExpMonth       CardLinkedIDType = "ExpMonth"
	CardLinkedIDTypeExpYear        CardLinkedIDType = "ExpYear"
	CardLinkedIDTypeNumber         CardLinkedIDType = "Number"
)

type IdentityLinkedIDType string

const (
	IdentityLinkedIDTypeAddress1       IdentityLinkedIDType = "Address1"
	IdentityLinkedIDTypeAddress2       IdentityLinkedIDType = "Address2"
	IdentityLinkedIDTypeAddress3       IdentityLinkedIDType = "Address3"
	IdentityLinkedIDTypeCity           IdentityLinkedIDType = "City"
	IdentityLinkedIDTypeCompany        IdentityLinkedIDType = "Company"
	IdentityLinkedIDTypeCountry        IdentityLinkedIDType = "Country"
	IdentityLinkedIDTypeEmail          IdentityLinkedIDType = "Email"
	IdentityLinkedIDTypeFirstName      IdentityLinkedIDType = "FirstName"
	IdentityLinkedIDTypeFullName       IdentityLinkedIDType = "FullName"
	IdentityLinkedIDTypeLastName       IdentityLinkedIDType = "LastName"
	IdentityLinkedIDTypeLicenseNumber  IdentityLinkedIDType = "LicenseNumber"
	IdentityLinkedIDTypeMiddleName     IdentityLinkedIDType = "MiddleName"
	IdentityLinkedIDTypePassportNumber IdentityLinkedIDType = "PassportNumber"
	IdentityLinkedIDTypePhone          IdentityLinkedIDType = "Phone"
	IdentityLinkedIDTypePostalCode     IdentityLinkedIDType = "PostalCode"
	IdentityLinkedIDTypeSsn            IdentityLinkedIDType = "Ssn"
	IdentityLinkedIDTypeState          IdentityLinkedIDType = "State"
	IdentityLinkedIDTypeTitle          IdentityLinkedIDType = "Title"
	IdentityLinkedIDTypeUsername       IdentityLinkedIDType = "Username"
)

