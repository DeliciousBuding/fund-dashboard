import { useQuery } from '@tanstack/react-query'
import { fetchFunds, fetchPortfolio, fetchSecurities, fetchPortfolioXirr, fetchExchangeRate } from '../api'

export function useFundsQuery(portfolioId: number) {
  return useQuery({
    queryKey: ['funds', portfolioId],
    queryFn: ({ signal }) => fetchFunds(portfolioId, signal),
    staleTime: 5 * 60 * 1000, // 5 min
  })
}

export function usePortfolioQuery(portfolioId: number) {
  return useQuery({
    queryKey: ['portfolio', portfolioId],
    queryFn: ({ signal }) => fetchPortfolio(portfolioId, signal),
    // NAV marks at most daily; align with funds/securities for occasional single-user use.
    staleTime: 5 * 60 * 1000,
  })
}

export function useSecuritiesQuery(portfolioId: number) {
  return useQuery({
    queryKey: ['securities', portfolioId],
    queryFn: ({ signal }) => fetchSecurities(portfolioId, signal),
    staleTime: 5 * 60 * 1000,
  })
}

export function usePortfolioXirrQuery(portfolioId: number) {
  return useQuery({
    queryKey: ['portfolioXirr', portfolioId],
    queryFn: ({ signal }) => fetchPortfolioXirr(portfolioId, signal),
    staleTime: 5 * 60 * 1000,
  })
}

export function useExchangeRateQuery() {
  return useQuery({
    queryKey: ['exchangeRate'],
    queryFn: ({ signal }) => fetchExchangeRate(signal),
    staleTime: 10 * 60 * 1000, // 10 min (exchange rate changes slowly)
  })
}

export function useAppQueries(portfolioId: number) {
  const funds = useFundsQuery(portfolioId)
  const portfolio = usePortfolioQuery(portfolioId)
  const securities = useSecuritiesQuery(portfolioId)
  const xirr = usePortfolioXirrQuery(portfolioId)
  const fx = useExchangeRateQuery()

  const isLoading = funds.isLoading || portfolio.isLoading || securities.isLoading
  const error = funds.error || portfolio.error || securities.error

  return { funds, portfolio, securities, xirr, fx, isLoading, error }
}
