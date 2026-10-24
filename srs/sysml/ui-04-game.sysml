// Рисунок А.6 — макет: страница игры (REQ-FUNC-015, REQ-FUNC-021…REQ-FUNC-024)
package UI_04_Game {
    private import UiKit::*;

    part <game> 'Страница игры' : Screen {
        part 'Панель навигации' : NavBar {
            part 'Вишлист' : Link;
            part 'Поиск' : Link;
            part 'Уведомления (2)' : Link;
            part 'Профиль' : Link;
        }
        part 'Сведения об игре' : Row {
            part '[обложка]' : Image;
            part 'Цена' : Column {
                part 'Hades' : Label;
                part '$12.49  −50 %  (обычно $24.99)' : Label;
                part 'Открыть в магазине Steam ↗' : Link;
            }
            part 'Исторический минимум' : Column {
                part '$9.99 · 22.11.2025 · источник: Steam' : Label;
                part 'Пометка: история неполная (менее 90 дней)' : Label;
            }
        }
        part 'Условие уведомления' : Row {
            part 'Скидка не менее N % ▾' : Select;
            part 'N [50]' : Input;
            part 'Сохранить' : Button;
        }
        part 'История цены в регионе BY' : Column {
            part 'Период' : Row {
                part '3 месяца' : Button;
                part '[1 год]' : Button;
                part 'Всё время' : Button;
            }
            part 'Ступенчатый график итоговой цены: ● исторический минимум, - - обычная цена' : Chart;
        }
        part 'Скидки в мире (по данным США, IsThereAnyDeal)' : ListView {
            part 'Распродажа 1' : ListItem {
                part '19.06.2026 – 03.07.2026' : Label;
                part '−50 %' : Label;
            }
            part 'Распродажа 2' : ListItem {
                part '18.12.2025 – 05.01.2026' : Label;
                part '−60 %' : Label;
            }
            part 'Распродажа 3' : ListItem {
                part '27.11.2025 – 03.12.2025' : Label;
                part '−40 %' : Label;
            }
        }
        part 'Подвал' : Footer {
            part 'Цены: магазин Steam · история: IsThereAnyDeal · не связано с Valve' : Label;
        }
    }
}
