using Moonwell;

namespace HeroLevel
{
    public sealed class HeroLevelMap : Map
    {
    }

    public class Veteran : Moonwell.Objects.Human.Units.Paladin
    {
        public Veteran()
        {
            Set("ulev", 3);
        }
    }
}
