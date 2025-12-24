import { useOutletContext } from 'react-router-dom';
import type { ContextType } from '~/common';
import { HeaderNewChat, OpenSidebar } from './Menus';
import { AnimatePresence, motion } from 'framer-motion';

export default function Header() {
  const { navVisible, setNavVisible } = useOutletContext<ContextType>();

  return (
    <div className="sticky top-0 z-10 flex h-14 w-full items-center justify-between bg-white p-2 font-semibold text-text-primary dark:!bg-[#111111]">
      <div className="hide-scrollbar flex w-full items-center justify-between gap-2 overflow-x-auto">
        <div className="mx-1 flex items-center">
          <AnimatePresence initial={false}>
            {!navVisible && (
              <motion.div
                className={`flex items-center gap-2`}
                initial={{ width: 0, opacity: 0 }}
                animate={{ width: 'auto', opacity: 1 }}
                exit={{ width: 0, opacity: 0 }}
                transition={{ duration: 0.2 }}
                key="header-buttons"
              >
                <OpenSidebar setNavVisible={setNavVisible} className="max-md:hidden" />
                <HeaderNewChat />
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Center: FIA Logo and Text */}
        <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 flex items-center justify-center gap-1">
          <div className="w-8 h-8 relative rounded-sm flex items-center justify-center">
            <img
              src="/assets/FIA.svg"
              alt="FIA"
              className="w-full h-full object-contain"
            />
          </div>
          <div 
            className="text-[20px] font-sans leading-5 text-text-primary dark:text-gray-100"
            style={{ fontSize: '20px' }}
          >
            FIA - FYERS Intelligent Assistant
          </div>
        </div>
      </div>
    </div>
  );
}
