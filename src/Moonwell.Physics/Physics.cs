using System;
using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>The physics package: its settings and the world it talks to. The package starts on its first use, and a map sets the settings in its Map constructor, before that.</summary>
    public static class Physics
    {
        /// <summary>The settings; made on first use.</summary>
        private static PhysicsSettings? settings;

        /// <summary>The world the package talks to; set by Install.</summary>
        private static IPhysicsWorld? host;

        /// <summary>The game's own world: the package installs itself on it at first use. Made from the natives on first use; a test sets a fake one to take the game's first-use path, and null after.</summary>
        internal static IPhysicsWorld? native;

        /// <summary>The package's hook before the gizmos' ticks, the physics pass; the package counts as installed while the interactions package's hook is this one. Interactions' own Install clears that hook, so this package sets it again after every install.</summary>
        private static Action? hook;

        /// <summary>The downward acceleration, in game units a second squared; 800. Read as a missile or a pushed unit moves, so a change takes hold at once.</summary>
        [LocalSafe]
        public static float Gravity
        {
            get { return Current().Gravity; }
            set { Current().Gravity = value; }
        }

        /// <summary>Whether the package is installed on a world.</summary>
        internal static bool IsInstalled
        {
            get { return Installed(); }
        }

        /// <summary>The world the package talks to. The package installs itself on the game first if it is not installed.</summary>
        internal static IPhysicsWorld World
        {
            get
            {
                EnsureInstalled();
                return host!;
            }
        }

        /// <summary>The game's own world, made from the natives on first use.</summary>
        private static IPhysicsWorld Native
        {
            get
            {
                if (native == null)
                {
                    native = new NativePhysicsWorld();
                }

                return native;
            }
        }

        /// <summary>Installs the package on a world; tests give a fake one. The game needs no call: the package installs itself on its native world on first use. The interactions package is installed first, on the game, if it is not; a test that gives it a fake world installs that first. A Runtime.Reset undoes the install.</summary>
        public static void Install(IPhysicsWorld world)
        {
            Interactions.EnsureInstalled();
            host = world;
            Pass.Reset();
            hook = Pass.Run;
            Tracking.BeforeGizmos = hook;
        }

        /// <summary>Forgets the settings and the world, so the next use starts from the defaults; the tests call it between cases.</summary>
        internal static void ResetSettings()
        {
            settings = null;
            host = null;
            hook = null;
            Pass.Reset();
        }

        /// <summary>Whether the package is installed: the interactions package is, and its hook before the gizmos is the package's own. Interactions' Install and Runtime.Reset clear the hook, so the next use starts afresh.</summary>
        private static bool Installed()
        {
            return hook != null && Interactions.IsInstalled && object.ReferenceEquals(Tracking.BeforeGizmos, hook);
        }

        /// <summary>Installs the package on the game's own world when it is not installed. Every first use calls it before it changes anything, as an install starts the package afresh.</summary>
        internal static void EnsureInstalled()
        {
            if (!Installed())
            {
                Install(Native);
            }
        }

        /// <summary>The settings, made with their defaults on first use. What runs many times a step reads a setting through it, a call less than the setting's getter.</summary>
        internal static PhysicsSettings Current()
        {
            if (settings == null)
            {
                settings = new PhysicsSettings();
            }

            return settings;
        }
    }
}
