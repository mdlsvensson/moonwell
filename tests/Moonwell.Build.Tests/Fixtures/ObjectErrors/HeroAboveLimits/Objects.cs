using Moonwell;

namespace HeroAboveLimits
{
    public sealed class HeroAboveLimitsMap : Map
    {
    }

    public class Champion : Moonwell.Objects.Human.Units.Paladin
    {
        public Champion()
        {
            MaxLife = 600000;
        }
    }
}
