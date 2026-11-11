import { createRoot } from 'react-dom/client'
import App from './app/App'
import { FeedbackProvider } from './components/feedback/FeedbackProvider'
import './styles.css'

createRoot(document.getElementById('root')!).render(<FeedbackProvider><App /></FeedbackProvider>)
