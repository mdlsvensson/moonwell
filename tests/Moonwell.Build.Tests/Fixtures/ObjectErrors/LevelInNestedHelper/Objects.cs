using Moonwell;

namespace LevelInNestedHelper
{
    public sealed class LevelInNestedHelperMap : Map
    {
    }

    public class Veteran : Moonwell.Objects.Human.Units.Footman
    {
        // A class of its own inside a declared class: its constructor runs in the game, where a unit's level has no setter.
        public sealed class Promoter
        {
            public Promoter(Unit recruit)
            {
                recruit.Level = 2;
            }
        }
    }
}
