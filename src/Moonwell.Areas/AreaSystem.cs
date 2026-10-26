using System;
using System.Collections.Generic;
using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>Starts the areas package; a map's generated Main calls Install when the map uses the package, with the World Editor regions the build wrote.</summary>
    public static class AreaSystem
    {
        /// <summary>The world the package talks to; null until Install.</summary>
        private static IAreaWorld? world;

        /// <summary>The World Editor regions by name, as bounds; empty until Install.</summary>
        private static readonly Dictionary<string, Bounds> regions = new Dictionary<string, Bounds>();

        /// <summary>Installs the package on the game with the map's World Editor regions: their names, and four floats for each (left, bottom, right, top), in the same order.</summary>
        public static void Install(string[] names, float[] rects)
        {
            Install(new NativeAreaWorld(), names, rects);
        }

        /// <summary>Installs the package on a world; tests give a fake one. Throws ArgumentException, and changes nothing, when the rects are not four floats for each name.</summary>
        public static void Install(IAreaWorld host, string[] names, float[] rects)
        {
            if (rects.Length != names.Length * 4)
            {
                throw new ArgumentException("Each region needs four floats: " + names.Length + " names and " + rects.Length + " floats.");
            }

            regions.Clear();
            for (var i = 0; i < names.Length; i++)
            {
                var a = new Vector2(rects[i * 4], rects[i * 4 + 1]);
                var b = new Vector2(rects[i * 4 + 2], rects[i * 4 + 3]);
                regions[names[i]] = new Bounds(a, b);
            }

            world = host;
        }

        /// <summary>The world the package talks to.</summary>
        internal static IAreaWorld World
        {
            get
            {
                if (world == null)
                {
                    throw NotInstalled();
                }

                return world;
            }
        }

#if DEBUG
        /// <summary>The world the package talks to, or null before Install; reading it throws nothing.</summary>
        internal static IAreaWorld? WorldIfInstalled
        {
            get { return world; }
        }
#endif

        /// <summary>Finds a World Editor region by name; false when the map has none of that name. Throws before Install.</summary>
        internal static bool TryRegion(string name, out Bounds bounds)
        {
            if (world == null)
            {
                throw NotInstalled();
            }

            return regions.TryGetValue(name, out bounds);
        }

        /// <summary>Forgets the install, so a case can check what reads before it do.</summary>
        internal static void Uninstall()
        {
            world = null;
            regions.Clear();
            AreaTracker.Forget();
        }

        /// <summary>The error of a read before Install.</summary>
        private static InvalidOperationException NotInstalled()
        {
            return new InvalidOperationException("Moonwell.Areas is not installed: the build installs it in a map that references the package");
        }
    }
}
