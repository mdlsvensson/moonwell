#pragma warning disable MW0015 // the analyzer reports this too; the fixture is for the build's own check
using Moonwell;

namespace LevelInOtherCtor
{
    public sealed class LevelInOtherCtorMap : Map
    {
    }

    // A map with no class of Moonwell.Objects; this one's constructor runs in the game, where a unit's level has no setter.
    public sealed class Drill
    {
        public Drill(Unit recruit)
        {
            recruit.Level = 2;
        }
    }
}
