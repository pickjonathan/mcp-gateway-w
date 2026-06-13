import { setupServer } from 'msw/node'
import { handlers } from './handlers'

// Node MSW server shared by component/contract/adversarial tests.
export const server = setupServer(...handlers)
