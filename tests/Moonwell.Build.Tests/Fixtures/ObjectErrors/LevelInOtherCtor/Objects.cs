using Moonwell;

namespace LevelInOtherCtor
{
    public sealed class LevelInOtherCtorMap : Map
    {
    }

    public class Recruit : Moonwell.Objects.Human.Units.Footman
    {
    }

    // Not a declared class: its constructor runs in the game, where a unit's level has no setter.
    public sealed class Drill
    {
        public Drill(Unit recruit)
        {
            recruit.Level = 2;
        }
    }
}
