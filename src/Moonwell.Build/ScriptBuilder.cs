using War3Net.Build;
using War3Net.Build.Widget;

namespace Moonwell.Build
{
    /// <summary>
    /// War3Net's script builder set for CSharp.lua, making the preplaced destructables as World Editor's script does: the game makes
    /// each one of <c>war3map.doo</c> that keeps its state's visible or solid bit, and the script makes the others (World Editor clears
    /// both on the ones its triggers name or that drop items). Those are made in a local, so no global is declared for any of them.
    /// </summary>
    internal sealed class ScriptBuilder : MapScriptBuilder
    {
        public ScriptBuilder()
            : base(triggerData: null!)
        {
            SetDefaultOptionsForCSharpLua();
            ForceGenerateGlobalDestructableVariable = false;
        }

        /// <summary>
        /// Hands the builder only the destructables the script makes while it builds, then gives the map back its own. The builder
        /// numbers a destructable's drop function by its place among the ones it makes and defines them by its place in the whole
        /// list, so the list it reads holds only those.
        /// </summary>
        /// <param name="map">The map.</param>
        /// <param name="build">The build, which reads the map.</param>
        /// <returns>What the build returns.</returns>
        public static T WithScriptDestructables<T>(Map map, Func<T> build)
        {
            var doodads = map.Doodads;
            if (doodads is null)
            {
                return build();
            }

            var made = new MapDoodads(doodads.FormatVersion, doodads.SubVersion, doodads.UseNewFormat)
            {
                Doodads = doodads.Doodads.Where(MadeByScript).ToList(),
                SpecialDoodadVersion = doodads.SpecialDoodadVersion,
                SpecialDoodads = doodads.SpecialDoodads,
            };
            map.Doodads = made;
            try
            {
                return build();
            }
            finally
            {
                map.Doodads = doodads;
            }
        }

        /// <summary>True for a destructable the game leaves to the script: neither its visible nor its solid bit is set.</summary>
        /// <param name="doodad">A destructable or doodad of <c>war3map.doo</c>.</param>
        public static bool MadeByScript(DoodadData doodad)
        {
            return (doodad.State & (DoodadState.NonSolidVisible | DoodadState.Normal)) == 0;
        }

        protected override bool ShouldGenerateDestructableVariable(DoodadData doodadData)
        {
            return true;
        }

        protected override bool ShouldGenerateDestructableVariables(Map map)
        {
            return false;
        }
    }
}
