import { HTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: 'default' | 'secondary' | 'success' | 'destructive' | 'warning' | 'outline'
}

export function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium transition-colors',
        variant === 'default' && 'bg-primary text-primary-foreground',
        variant === 'secondary' && 'bg-secondary text-secondary-foreground',
        variant === 'success' && 'bg-green-100 text-green-800',
        variant === 'destructive' && 'bg-destructive text-destructive-foreground',
        variant === 'warning' && 'bg-yellow-100 text-yellow-800',
        variant === 'outline' && 'border text-foreground',
        className
      )}
      {...props}
    />
  )
}
