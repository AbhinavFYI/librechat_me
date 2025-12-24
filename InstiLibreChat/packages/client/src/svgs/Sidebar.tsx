export default function Sidebar({ className }: { className?: string }) {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 16 16"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
    >
      <path
        d="M12.7969 5.20156L10.0102 7.97656L12.7969 10.7516L11.95 11.6023L8.73594 8.40234L8.30937 7.97656L11.9508 4.35156L12.7969 5.20156Z"
        fill="currentColor"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M16 16H0V0H16V16ZM1.2 14.8H5.2V1.2H1.2V14.8ZM6.4 14.6H14.8V1.4H6.4V14.6Z"
        fill="currentColor"
      />
    </svg>
  );
}
