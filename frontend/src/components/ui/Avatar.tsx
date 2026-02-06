type AvatarSize = 'sm' | 'md' | 'lg'

interface AvatarProps {
  name: string
  image?: string
  size?: AvatarSize
}

/**
 * Avatar component with initials fallback and color based on name hash
 * Circular design matching incident.io patterns
 */
export function Avatar({ name, image, size = 'md' }: AvatarProps) {
  const sizeStyles = {
    sm: 'h-6 w-6 text-xs',
    md: 'h-8 w-8 text-sm',
    lg: 'h-10 w-10 text-base',
  }

  const initials = name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  // Generate consistent color from name hash
  const colorFromName = (str: string) => {
    let hash = 0
    for (let i = 0; i < str.length; i++) {
      hash = str.charCodeAt(i) + ((hash << 5) - hash)
    }

    const colors = [
      'bg-blue-500',
      'bg-purple-500',
      'bg-pink-500',
      'bg-red-500',
      'bg-orange-500',
      'bg-green-500',
      'bg-teal-500',
      'bg-cyan-500',
    ]

    return colors[Math.abs(hash) % colors.length]
  }

  if (image) {
    return (
      <img
        src={image}
        alt={name}
        className={`${sizeStyles[size]} rounded-full object-cover`}
      />
    )
  }

  return (
    <div
      className={`${sizeStyles[size]} ${colorFromName(name)} rounded-full flex items-center justify-center text-white font-medium`}
      title={name}
    >
      {initials}
    </div>
  )
}
