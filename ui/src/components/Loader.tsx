import Lottie from 'lottie-react'
import loader from '../../public/loader.json'

export default function Loader() {
  return (
    <div className="w-6 h-6">
      <Lottie animationData={loader} loop autoplay />
    </div>
  )
}
