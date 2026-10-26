using System;
using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>The interactions package: its settings, and the world and grid it keeps. The package starts on its first use, and a map sets the settings in its Map constructor, before that.</summary>
    public static class Interactions
    {
        /// <summary>The settings; made on first use.</summary>
        private static Settings? settings;

        /// <summary>The world the package talks to; set by Install. The cycle and a search, which run only once the package is installed, read it here rather than through World, which checks the install at every read.</summary>
        internal static IInteractionWorld? host;

        /// <summary>The grid; made on first use after an Install. The cycle and a search read it here rather than through Grid, which checks the install at every read, and take Grid when it is not made.</summary>
        internal static Grid? grid;

        /// <summary>The game's own world: the package installs itself on it at first use, and the reads that must make nothing ask it. Made from the natives on first use; a test sets a fake one to take the game's first-use path, and null after.</summary>
        internal static IInteractionWorld? native;

        /// <summary>The package's handler of core's TakenIn hook; the package counts as installed while that hook is this one.</summary>
        private static Action<Widget>? handler;

        /// <summary>The side of a grid cell, in game units, more than 0; 128. Read when the grid is made.</summary>
        [LocalSafe]
        public static float CellSize
        {
            get { return Current().CellSize; }
            set
            {
                Unlocked();
                Dev.Check(value > 0.0f, "Interactions.CellSize: must be more than 0");
                Current().CellSize = value;
            }
        }

        /// <summary>How often a unit's or item's cell is read again, in seconds, more than 0; 0.1. Read when the grid is made.</summary>
        [LocalSafe]
        public static float CellCheck
        {
            get { return Current().CellCheck; }
            set
            {
                Unlocked();
                Dev.Check(value > 0.0f, "Interactions.CellCheck: must be more than 0");
                Current().CellCheck = value;
            }
        }

        /// <summary>How far past an interaction's range a check looks, in game units, to cover what a unit moves between two reads of its cell; 64. Read when the grid is made.</summary>
        [LocalSafe]
        public static float Slack
        {
            get { return Current().Slack; }
            set
            {
                Unlocked();
                Current().Slack = value;
            }
        }

        /// <summary>The length of a step of the cycle, in seconds, more than 0; 1/32. Read when the grid is made.</summary>
        [LocalSafe]
        public static float Step
        {
            get { return Current().Step; }
            set
            {
                Unlocked();
                Dev.Check(value > 0.0f, "Interactions.Step: must be more than 0");
                Current().Step = value;
            }
        }

        /// <summary>The grid, made from the world's bounds on first use. The package installs itself on the game first if it is not installed.</summary>
        internal static Grid Grid
        {
            get
            {
                EnsureInstalled();
                if (grid == null)
                {
                    host!.WorldBounds(out var minX, out var minY, out var maxX, out var maxY);
                    grid = new Grid(minX, minY, maxX, maxY, Current().CellSize);
                }

                return grid;
            }
        }

        /// <summary>The grid when the package is installed and it has been made; null otherwise. Unlike Grid it installs nothing and makes nothing, for a search inside Local.For, which may change no shared state.</summary>
        internal static Grid? GridIfMade
        {
            get { return Installed() ? grid : null; }
        }

        /// <summary>Whether the package is installed on a world.</summary>
        internal static bool IsInstalled
        {
            get { return Installed(); }
        }

        /// <summary>The world a read of a place asks the game through. It is the package's world when it is installed; otherwise the game's natives, without installing the package, for a search inside Local.For, which may change no shared state.</summary>
        internal static IInteractionWorld PeekWorld
        {
            get
            {
                return Installed() ? host! : Native;
            }
        }

        /// <summary>The game's own world, made from the natives on first use.</summary>
        private static IInteractionWorld Native
        {
            get
            {
                if (native == null)
                {
                    native = new NativeInteractionWorld();
                }

                return native;
            }
        }

        /// <summary>Forgets the settings, so the next use starts from the defaults; the tests call it between cases.</summary>
        internal static void ResetSettings()
        {
            settings = null;
        }

        /// <summary>The world the package talks to. The package installs itself on the game first if it is not installed.</summary>
        internal static IInteractionWorld World
        {
            get
            {
                EnsureInstalled();
                return host!;
            }
        }

        /// <summary>Installs the package on a world, with a grid of its own to be made on first use; tests give a fake one. The game needs no call: the package installs itself on its native world on first use. A Runtime.Reset undoes the install.</summary>
        public static void Install(IInteractionWorld world)
        {
            host = world;
            grid = null;
            Tracking.Reset();
            handler = TakenIn;
            Hooks.TakenIn = handler;
            Hooks.Ending = Ending;
        }

        /// <summary>Whether the package is installed: core's TakenIn hook is the package's own. Runtime.Reset clears the hook, so the next use starts afresh.</summary>
        private static bool Installed()
        {
            return handler != null && object.ReferenceEquals(Hooks.TakenIn, handler);
        }

        /// <summary>Installs the package on the game's own world when it is not installed. Every first use calls it before it changes anything, as an install starts the package afresh.</summary>
        internal static void EnsureInstalled()
        {
            if (!Installed())
            {
                Install(Native);
            }
        }

        /// <summary>The settings, made with their defaults on first use.</summary>
        private static Settings Current()
        {
            if (settings == null)
            {
                settings = new Settings();
            }

            return settings;
        }

        /// <summary>Fails in a development build when the grid exists, as the settings are read when it is made.</summary>
        private static void Unlocked()
        {
            Dev.Check(grid == null || !Installed(), "Interactions: settings are read when the grid is made; set them in the map's constructor");
        }

        /// <summary>Core's hook for a widget taken in: it is tracked when its kind has been asked for.</summary>
        private static void TakenIn(Widget widget)
        {
            Tracking.TakenIn(widget);
        }

        /// <summary>Core's hook for a widget ending: it is out of the grid at once, before its OnLeave runs. A gizmo, the package's own, leaves the grid after its OnLeave.</summary>
        private static void Ending(Widget widget)
        {
            Tracking.WidgetEnding(widget);
        }
    }
}
