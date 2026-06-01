import * as React from 'react';

import { cn } from '@/lib/utils';

const Label = React.forwardRef<HTMLLabelElement, React.LabelHTMLAttributes<HTMLLabelElement>>(({ className, ...props }, ref) => {
  return <label ref={ref} className={cn('text-sm font-medium leading-none text-foreground', className)} {...props} />;
});
Label.displayName = 'Label';

export { Label };
