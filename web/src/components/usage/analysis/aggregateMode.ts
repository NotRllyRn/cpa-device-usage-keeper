import { createContext, useContext } from 'react';

// Opt-in only. The normal Keeper request analysis remains unchanged.
export const AnalysisAggregateContext = createContext(false);
export const useAnalysisAggregate = () => useContext(AnalysisAggregateContext);
