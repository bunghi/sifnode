import detectMetaMaskProvider from "@metamask/detect-provider";
import Web3 from "web3";
import { AbstractProvider, provider } from "web3-core";
// import notify from "../../utils/Notifications"

type MetaMaskProvider = AbstractProvider & {
  request?: (a: any) => Promise<void>;
};

type WindowWithPossibleMetaMask = typeof window & {
  ethereum?: MetaMaskProvider;
  web3?: Web3;
};

// Detect mossible metamask provider from browser
export const getMetamaskProvider = async (): Promise<provider> => {
  const mmp = await detectMetaMaskProvider();
  const win = window as WindowWithPossibleMetaMask;
  if (!mmp) {
    // XXX: Should not have access to sideeffects here surface in business layer
    // TODO: Trigger this notifications in usecases
    // This is tricky because of the way the ethereumService is designed and this needs to be overhauled
    // Usecase Layer should do something along the lines of:

    // ```ts
    // const provider:IBlockchainProvider | null = MetamaskProvider.create()
    // if(!provider){
    //   services.events.dispatch({type:'MetamaskNotFoundErrorEvent'}) }
    //   return
    // }
    // services.eth.setProvider(provider)
    // ```

    // notify({
    //   type: "error",
    //   message: "Metamask not found.",
    //   detail: {
    //     type: "info",
    //     message: "Check if extension enabled for this URL.",
    //   },
    // });
    return null;
  }
  if (!win) return null;
  if (mmp) {
    return mmp as provider;
  }

  // if a wallet has left web3 on the page we can use the current provider
  if (win.web3) {
    return win.web3.currentProvider;
  }

  return null;
};
