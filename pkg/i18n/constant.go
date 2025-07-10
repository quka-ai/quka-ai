package i18n

var ALLOW_LANG = map[string]bool{
	"en":    true,
	"zh-CN": true,
}

const DEFAULT_LANG = "en"

const (
	ERROR_INTERNAL                   = "error.internal"
	ERROR_NOT_FOUND                  = "error.notfound"
	ERROR_INVALIDARGUMENT            = "error.invalidargument"
	ERROR_USER_SPACE_NOT_FOUND       = "error.user_space_not_found"
	ERROR_PERMISSION_DENIED          = "error.permission.denied"
	ERROR_UNAUTHORIZED               = "error.unauthorized"
	ERROR_PAYMENT_REQUIRED           = "error.payment_required"
	ERROR_EXIST                      = "error.exist"
	ERROR_TITLE_EXIST                = "error.title.exist"
	ERROR_FORBIDDEN                  = "error.forbidden"
	ERROR_TOO_MANY_REQUESTS          = "error.tooManyRequests"
	ERROR_MORE_TAHN_MAX              = "error.moreThanMax"
	ERROR_UNSUPPORTED_FEATURE        = "error.unsupported.feature"
	ERROR_VERIFY_CODE_ALREADY_SENDED = "error.verifycodesended"
	ERROR_RESET_EMAIL_ALREADY_SENDED = "error.reset_email_sended"
	ERROR_VERIFY_CODE_INCORRECT      = "error.incorrect.verifycode"
	ERROR_VERIFY_CODE_EXPIRED        = "error.incorrect.verifycode.expired"
	ERROR_LOGIN_ACCOUNT_INCORRECT    = "error.login.account.incorrect"
	ERROR_EMAIL_ALREADY_REGISTED     = "error.email_has_already_registed"
	ERROR_EMAIL_NOT_MATCH            = "error.email_not_match"
	ERROR_EMAIL_NOT_REGISTERED       = "error.email_not_registered"
	ERROR_ALREADY_INVITED            = "error.already_invited"
	ERROR_ALREADY_SAVED              = "error.already_saved"
	ERROR_INEFFECTIVE                = "error.ineffective"
	ERROR_REDEEM_MUST_NEW_USER       = "error.redeem.must_new_user"
	ERROR_ALREADY_APPLIED            = "error.already_applied"
	ERROR_IMAGE_READ_FAIL            = "error.image.read_file"
	ERROR_IMAGE_TYPE_UNSUPPORT       = "error.image.type.unsupport"

	ERROR_INVALID_TOKEN   = "error.invalid.token"
	ERROR_INVALID_ACCOUNT = "error.invalid.account"

	ERROR_LOGIC_VECTOR_DB_NOT_MATCHED_CONTENT_DB = "error.logic.vector.db.notmatch.content.db"
	ERROR_PROVIDER_MODEL_IN_USE                  = "error.provider.model.in.use"
	ERROR_AI_CHAT_MODEL_NOT_FOUND                = "error.ai.chat.model.not.found"
	ERROR_AI_EMBEDDING_MODEL_NOT_FOUND           = "error.ai.embedding.model.not.found"

	MESSAGE_AI_CONFIG_RELOAD_SUCCESS = "message.ai.config.reload.success"
	MESSAGE_AI_USAGE_UPDATE_SUCCESS  = "message.ai.usage.update.success"
)
