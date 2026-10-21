using Moonwell;

namespace HeroBelowAttributes
{
    public sealed class HeroBelowAttributesMap : Map
    {
    }

    public class Champion : Moonwell.Objects.Human.Units.Paladin
    {
        public Champion()
        {
            MaxLife = 400;
            Strength = 30;
        }
    }
}
