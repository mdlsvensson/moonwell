using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>Starts the damage package; a map's generated Main calls Install when the map uses the package.</summary>
    public static class DamageSystem
    {
        /// <summary>Installs the package on the game, before anything registers.</summary>
        public static void Install()
        {
            Install(new NativeDamageWorld());
        }

        /// <summary>Installs the package on a world; tests give a fake one.</summary>
        public static void Install(IDamageWorld world)
        {
            Pipeline.Install(world);
        }
    }
}
