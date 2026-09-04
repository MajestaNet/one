import logoGold from "../../../assets/brand/logo-gold.svg";
import logoTwoColour from "../../../assets/brand/logo-two-colour.svg";
import symbolGold from "../../../assets/brand/symbol-gold.svg";
import symbolNavy from "../../../assets/brand/symbol-navy.svg";
import { useTheme } from "../ThemeContext";

export type BrandMarkVariant = "symbol" | "lockup";

/**
 * Sourced Majesta.Net mark. Compact globe for the top bar; full lockup
 * (min 180px) for Sign in and the mode launcher. Do not type the wordmark.
 */
export function BrandMark({
  variant,
  product = true,
}: {
  variant: BrandMarkVariant;
  product?: boolean;
}) {
  const theme = useTheme();
  const dark = theme === "dark";
  const compact = variant === "symbol";
  const src = compact ? (dark ? symbolGold : symbolNavy) : dark ? logoGold : logoTwoColour;
  const testId = compact ? "brand" : "brand-lockup";
  const className = compact ? "brand brand-symbol" : "brand brand-lockup mode-launcher-brand";

  return (
    <p className={className} data-testid={testId}>
      <img src={src} alt="Majesta.Net" />
      {product ? <span className="brand-product">Control</span> : null}
    </p>
  );
}
