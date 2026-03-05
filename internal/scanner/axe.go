package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const axeCoreCDN = "https://cdnjs.cloudflare.com/ajax/libs/axe-core/4.10.2/axe.min.js"

// InjectAxeCore loads axe-core into the current page via CDN.
func InjectAxeCore(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("InjectAxeCore: context is nil")
	}

	injectScript := fmt.Sprintf(`(() => {
        const existing = document.querySelector('script[data-axe-core="true"]');
        if (existing) return true;
        const script = document.createElement('script');
        script.src = %q;
        script.async = true;
        script.setAttribute('data-axe-core', 'true');
        document.head.appendChild(script);
        return true;
    })();`, axeCoreCDN)

	if err := chromedp.Run(ctx, chromedp.Evaluate(injectScript, nil)); err != nil {
		return fmt.Errorf("InjectAxeCore: inject script: %w", err)
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		var loaded bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`typeof axe === 'object'`, &loaded)); err != nil {
			return fmt.Errorf("InjectAxeCore: check loaded: %w", err)
		}
		if loaded {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("InjectAxeCore: timeout waiting for axe-core: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// ExecuteAxe runs axe-core against the current page and returns violations.
func ExecuteAxe(ctx context.Context, excludeRules []string) ([]Violation, error) {
	if ctx == nil {
		return nil, fmt.Errorf("ExecuteAxe: context is nil")
	}

	rules := make(map[string]map[string]bool)
	for _, rule := range excludeRules {
		if rule == "" {
			return nil, fmt.Errorf("ExecuteAxe: exclude rule is empty")
		}
		rules[rule] = map[string]bool{"enabled": false}
	}

	options := map[string]any{}
	if len(rules) > 0 {
		options["rules"] = rules
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("ExecuteAxe: marshal options: %w", err)
	}

	js := fmt.Sprintf(`(async () => {
	const options = %s;
	const results = await axe.run(document, options);
	return JSON.stringify(results);
})()`, string(optionsJSON))

	var resultJSON string
	evalAwaitPromise := chromedp.EvaluateOption(func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true).WithReturnByValue(true)
	})
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &resultJSON, evalAwaitPromise)); err != nil {
		return nil, fmt.Errorf("ExecuteAxe: run axe: %w", err)
	}
	if resultJSON == "" {
		return nil, fmt.Errorf("ExecuteAxe: empty results")
	}

	var results struct {
		Violations []Violation `json:"violations"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &results); err != nil {
		return nil, fmt.Errorf("ExecuteAxe: parse results: %w", err)
	}

	return results.Violations, nil
}
