import Image from "next/image";
import { homeAssets } from "../../shared/header";

/** Both theme lockups; shared/brand.css shows the one matching html.dark. */
export function BrandLogo({ priority = false }: { priority?: boolean }) {
  return (
    <span className="wk-logo">
      <Image className="wk-logo-light" src={`${homeAssets}/brand/ai-kb-lockup-light.png`} alt="AI KB" width={945} height={238} priority={priority} />
      <Image className="wk-logo-dark" src={`${homeAssets}/brand/ai-kb-lockup-dark.png`} alt="AI KB" width={945} height={238} priority={priority} />
    </span>
  );
}
