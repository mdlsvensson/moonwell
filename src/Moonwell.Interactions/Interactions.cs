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

        /// <summary>The world the package talks to; set by Install.</summary>
        private static IInteractionWorld? host;

        /// <summary>The grid; made on first use after an Install.</summary>
        private static Grid? grid;

        /// <summary>The package's handler of core's TakenIn hook; the package counts as installed while that hook is this one.</summary>
        private static Action<Widget>? handler;

        /// <summary>The side of a grid cell, in game units; 256. Read when the grid is made.</summary>
        public static float CellSize
        {
            get { return Current().CellSize; }
            set
            {
                Unlocked();
                Current().CellSize = value;
            }
        }

        /// <summary>How often a unit's or item's cell is read again, in seconds; 0.1. Read when the grid is made.</summary>
        public static float CellCheck
        {
            get { return Current().CellCheck; }
            set
            {
                Unlocked();
                Current().CellCheck = value;
            }
        }

        /// <summary>How far past an interaction's range a check looks, in game units, to cover what a unit moves between two reads of its cell; 64. Read when the grid is made.</summary>
        public static float Slack
        {
            get { return Current().Slack; }
            set
            {
                Unlocked();
                Current().Slack = value;
            }
        }

        /// <summary>The length of a step of the cycle, in seconds; 1/32. Read when the grid is made.</summary>
        public static float Step
        {
            get { return Current().Step; }
            set
            {
                Unlocked();
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
            handler = TakenIn;
            Hooks.TakenIn = handler;
        }

        /// <summary>Whether the package is installed: core's TakenIn hook is the package's own. Runtime.Reset clears the hook, so the next use starts afresh.</summary>
        private static bool Installed()
        {
            return handler != null && object.ReferenceEquals(Hooks.TakenIn, handler);
        }

        /// <summary>Installs the package on the game's natives when it is not installed.</summary>
        private static void EnsureInstalled()
        {
            if (!Installed())
            {
                Install(new NativeInteractionWorld());
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

        /// <summary>Core's hook for a widget taken in. It does nothing yet.</summary>
        private static void TakenIn(Widget widget)
        {
        }
    }
}
