using Moonwell;

namespace OutOfRangeLevel
{
    public sealed class OutOfRangeLevelMap : Map
    {
    }

    public class Bolt : Moonwell.Objects.Human.Abilities.StormBolt
    {
        public Bolt()
        {
            Cooldown = PerLevel(level => 1300f * level);
        }
    }
}
