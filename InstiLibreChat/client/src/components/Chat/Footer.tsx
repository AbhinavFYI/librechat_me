import React, { useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import TagManager from 'react-gtm-module';
import { Constants } from 'librechat-data-provider';
import { useGetStartupConfig } from '~/data-provider';
import { useLocalize } from '~/hooks';

export default function Footer({ className }: { className?: string }) {
  const { data: config } = useGetStartupConfig();
  const localize = useLocalize();

  const privacyPolicy = config?.interface?.privacyPolicy;
  const termsOfService = config?.interface?.termsOfService;

  const privacyPolicyRender = privacyPolicy?.externalUrl != null && (
    <a
      className="text-text-secondary underline"
      href={privacyPolicy.externalUrl}
      target={privacyPolicy.openNewTab === true ? '_blank' : undefined}
      rel="noreferrer"
    >
      {localize('com_ui_privacy_policy')}
    </a>
  );

  const termsOfServiceRender = termsOfService?.externalUrl != null && (
    <a
      className="text-text-secondary underline"
      href={termsOfService.externalUrl}
      target={termsOfService.openNewTab === true ? '_blank' : undefined}
      rel="noreferrer"
    >
      {localize('com_ui_terms_of_service')}
    </a>
  );

  const mainContentParts = (
    localize('com_ui_latest_footer')
  ).split('|');

  useEffect(() => {
    if (config?.analyticsGtmId != null && typeof window.google_tag_manager === 'undefined') {
      const tagManagerArgs = {
        gtmId: config.analyticsGtmId,
      };
      TagManager.initialize(tagManagerArgs);
    }
  }, [config?.analyticsGtmId]);

  const mainContentRender = (
    <div className="flex flex-col items-center justify-center text-center">
      {mainContentParts.map((text, index) => (
        <ReactMarkdown
          key={`main-content-part-${index}`}
          components={{
            a: ({ node: _n, href, children, ...otherProps }) => {
              return (
                <a
                  className="text-text-secondary underline"
                  href={href}
                  target="_blank"
                  rel="noreferrer"
                  {...otherProps}
                >
                  {children}
                </a>
              );
            },

            p: ({ node: _n, ...props }) => <span {...props} />,
          }}
        >
          {text.trim()}
        </ReactMarkdown>
      ))}
    </div>
  );

  const footerElements = [privacyPolicyRender, termsOfServiceRender].filter(Boolean);

  return (
    <div className="relative w-full">
      <div
        className={
          className ??
          'absolute bottom-0 left-0 right-0 hidden flex-col items-center justify-center gap-2 px-2 py-2 text-center text-xs text-text-primary sm:flex md:px-[60px]'
        }
        role="contentinfo"
      >
        {/* Main content with line breaks */}
        {mainContentRender}
        
        {/* Privacy & Terms links */}
        {footerElements.length > 0 && (
          <div className="flex items-center gap-2 mt-1">
            {footerElements.map((contentRender, index) => {
              const isLastElement = index === footerElements.length - 1;
              return (
                <React.Fragment key={`footer-element-${index}`}>
                  {contentRender}
                  {!isLastElement && (
                    <div
                      key={`separator-${index}`}
                      className="h-2 border-r-[1px] border-border-medium"
                    />
                  )}
                </React.Fragment>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
