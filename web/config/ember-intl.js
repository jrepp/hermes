module.exports = function (/* environment */) {
  return {
    // Locale to use if the user's locale is not supported
    fallbackLocale: 'en-us',
    
    // Locale to use by default
    defaultLocale: 'en-us',
    
    // Don't throw errors on missing translations
    errorOnMissingTranslations: false,
    
    // Don't warn on missing translations in development
    requiresTranslation(/* key, locale */) {
      return false;
    },
  };
};
