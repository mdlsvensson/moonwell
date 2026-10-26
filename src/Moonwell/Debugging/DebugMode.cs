using System.Diagnostics;

namespace Moonwell
{
    /// <summary>
    /// The debug mode as map code sees it: whether it is on, the selected object, and a selection or a halt from code. In a
    /// release build IsOn and IsSelected read false, Selected null, and the compiler takes calls to Select and Halt out.
    /// </summary>
    public static class DebugMode
    {
        /// <summary>Whether the debug mode is on (-debug); false in a release build.</summary>
        [LocalSafe]
        public static bool IsOn
        {
            get
            {
#if DEBUG
                return DebugSession.IsOn;
#else
                return false;
#endif
            }
        }

        /// <summary>The object selected in the debug mode, the same for every player; null when none, while off, and in a release build.</summary>
        [LocalSafe]
        public static Actor? Selected
        {
            get
            {
#if DEBUG
                return DebugSession.Selected;
#else
                return null;
#endif
            }
        }

        /// <summary>Whether an object is the one selected in the debug mode; false in a release build.</summary>
        [LocalSafe]
        public static bool IsSelected(Actor actor)
        {
#if DEBUG
            return DebugSession.IsOn && DebugSession.Selected == actor;
#else
            return false;
#endif
        }

        /// <summary>Selects an object in the debug mode, or nothing with null, as a Ctrl + click does; nothing while the mode is off. A release build leaves the call out.</summary>
        [Conditional("DEBUG")]
        public static void Select(Actor? actor)
        {
#if DEBUG
            DebugSession.Select(actor);
#endif
        }

        /// <summary>Halts Moonwell and holds every unit, as Ctrl + H does, like a breakpoint: the code after the call still runs, and what is due after it waits for a step or a resume. Nothing while the mode is off or already halted. A release build leaves the call out.</summary>
        [Conditional("DEBUG")]
        public static void Halt()
        {
#if DEBUG
            DebugSession.Halt(true);
#endif
        }
    }
}
