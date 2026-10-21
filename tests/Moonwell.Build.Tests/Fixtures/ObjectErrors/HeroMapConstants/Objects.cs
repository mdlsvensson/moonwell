using Moonwell;

namespace HeroMapConstants
{
    public sealed class HeroMapConstantsMap : Map
    {
    }

    // The map's gameplay constants give 10 life a point of strength: the Paladin's 22 give 220.
    public class Champion : Moonwell.Objects.Human.Units.Paladin
    {
        public Champion()
        {
            MaxLife = MaxLife + 100;
            Strength = 30;
        }
    }
}
