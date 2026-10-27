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

        /// <summary>The object selected in the debug mode, the same for every player; null when none, once it has ended, while off, and in a release build.</summary>
        [LocalSafe]
        public static Actor? Selected
        {
            get
            {
#if DEBUG
                // The panel lets go of an ended object at its next run; until then it reads as none here.
                var selected = DebugSession.Selected;
                return selected != null && !selected.HasEnded ? selected : null;
#else
                return null;
#endif
            }
        }

        /// <summary>Whether an object is the one selected in the debug mode; false once it has ended, and in a release build.</summary>
        [LocalSafe]
        public static bool IsSelected(Actor actor)
        {
#if DEBUG
            return DebugSession.IsOn && DebugSession.Selected == actor && !actor.HasEnded;
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

        /// <summary>Halts Moonwell and holds every unit, as Ctrl + H does, like a breakpoint: the code after the call still runs, and what is due after it waits for a step or a resume. While the mode is off it is turned on first, as -debug does, unless -debug turned it off: then nothing until the mode is turned on again, which escapes a breakpoint whose condition still holds. Nothing when already halted. A release build leaves the call out.</summary>
        [Conditional("DEBUG")]
        public static void Halt()
        {
#if DEBUG
            DebugSession.HaltFromCode();
#endif
        }
    }
}
