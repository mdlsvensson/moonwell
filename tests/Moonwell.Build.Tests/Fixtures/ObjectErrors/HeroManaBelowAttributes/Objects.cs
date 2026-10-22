using Moonwell;

namespace HeroManaBelowAttributes
{
    public sealed class HeroManaBelowAttributesMap : Map
    {
    }

    public class Sage : Moonwell.Objects.Human.Units.Paladin
    {
        public Sage()
        {
            Mana = 200;
        }
    }
}
