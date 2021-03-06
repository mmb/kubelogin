package authentication

import (
	"context"

	"github.com/int128/kubelogin/pkg/adaptors/browser"
	"github.com/int128/kubelogin/pkg/adaptors/logger"
	"github.com/int128/kubelogin/pkg/adaptors/oidcclient"
	"github.com/int128/kubelogin/pkg/domain/oidc"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

// AuthCode provides the authentication code flow.
type AuthCode struct {
	Browser browser.Interface
	Logger  logger.Interface
}

func (u *AuthCode) Do(ctx context.Context, o *AuthCodeOption, client oidcclient.Interface) (*Output, error) {
	u.Logger.V(1).Infof("performing the authentication code flow")
	state, err := oidc.NewState()
	if err != nil {
		return nil, xerrors.Errorf("could not generate a state: %w", err)
	}
	nonce, err := oidc.NewNonce()
	if err != nil {
		return nil, xerrors.Errorf("could not generate a nonce: %w", err)
	}
	p, err := oidc.NewPKCEParams()
	if err != nil {
		return nil, xerrors.Errorf("could not generate PKCE parameters: %w", err)
	}
	in := oidcclient.GetTokenByAuthCodeInput{
		BindAddress:         o.BindAddress,
		State:               state,
		Nonce:               nonce,
		CodeChallenge:       p.CodeChallenge,
		CodeChallengeMethod: p.CodeChallengeMethod,
		CodeVerifier:        p.CodeVerifier,
	}
	readyChan := make(chan string, 1)
	defer close(readyChan)
	var out Output
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		select {
		case url, ok := <-readyChan:
			if !ok {
				return nil
			}
			if o.SkipOpenBrowser {
				u.Logger.Printf("Please visit the following URL in your browser: %s", url)
				return nil
			}
			if err := u.Browser.Open(url); err != nil {
				u.Logger.Printf(`error: could not open the browser: %s

Please visit the following URL in your browser manually: %s`, err, url)
				return nil
			}
			return nil
		case <-ctx.Done():
			return xerrors.Errorf("context cancelled while waiting for the local server: %w", ctx.Err())
		}
	})
	eg.Go(func() error {
		tokenSet, err := client.GetTokenByAuthCode(ctx, in, readyChan)
		if err != nil {
			return xerrors.Errorf("authorization code flow error: %w", err)
		}
		out = Output{
			IDToken:       tokenSet.IDToken,
			IDTokenClaims: tokenSet.IDTokenClaims,
			RefreshToken:  tokenSet.RefreshToken,
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, xerrors.Errorf("authentication error: %w", err)
	}
	return &out, nil
}
